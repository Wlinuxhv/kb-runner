package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"kb-runnerx/internal/adapter"
	"kb-runnerx/pkg/config"
	"kb-runnerx/pkg/logger"
	"kb-runnerx/pkg/result"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSuccess   Status = "success"
	StatusFailure   Status = "failure"
	StatusTimeout   Status = "timeout"
	StatusCancelled Status = "cancelled"
)

type ExecutionResult struct {
	TaskID       string
	Status       Status
	Steps        []*result.StepResult
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
	Output       string
	ResultJSON   string
	ErrorMessage string
	Score        float64
}

type ExecutionStatus struct {
	ExecutionID string
	Status      Status
	Tasks       []*adapter.Task
	Results     []*ExecutionResult
	StartTime   time.Time
	EndTime     time.Time
}

type Engine struct {
	cfg        *config.Config
	log        *logger.Logger
	registry   *adapter.Registry
	tasks      map[string]*ExecutionStatus
	mu         sync.RWMutex
}

func NewEngine(cfg *config.Config, log *logger.Logger) *Engine {
	return &Engine{
		cfg:      cfg,
		log:      log,
		registry: adapter.NewRegistry(),
		tasks:    make(map[string]*ExecutionStatus),
	}
}

func (e *Engine) RegisterAdapter(language adapter.Language, a adapter.Adapter) {
	e.registry.Register(language, a)
}

func (e *Engine) Execute(ctx context.Context, task *adapter.Task) (*ExecutionResult, error) {
	taskID := task.ID
	if taskID == "" {
		taskID = generateID()
		task.ID = taskID
	}

	if task.Timeout == 0 {
		task.Timeout = e.cfg.Execution.Timeout
	}

	if task.WorkDir == "" {
		task.WorkDir = e.cfg.Execution.WorkDir
	}

	if task.Weight == 0 {
		task.Weight = e.cfg.GetScriptWeight(task.ScriptName)
	}

	adapt, err := e.registry.Get(task.Language)
	if err != nil {
		return nil, fmt.Errorf("failed to get adapter: %w", err)
	}

	if err := adapt.Validate(task.ScriptPath); err != nil {
		return nil, fmt.Errorf("script validation failed: %w", err)
	}

	if err := adapt.PrepareEnvironment(task); err != nil {
		return nil, fmt.Errorf("failed to prepare environment: %w", err)
	}

	execCtx, cancel := context.WithTimeout(ctx, task.Timeout)
	defer cancel()

	e.log.Info("Starting task execution",
		"task_id", taskID,
		"script", task.ScriptPath,
		"language", task.Language,
		"timeout", task.Timeout,
	)

	adapterResult, err := adapt.Execute(execCtx, task)
	if err != nil {
		e.log.Error("Task execution failed",
			"task_id", taskID,
			"error", err,
		)
		return nil, err
	}

	execResult := &ExecutionResult{
		TaskID:       adapterResult.TaskID,
		Status:       convertStatus(adapterResult.Status),
		Steps:        adapterResult.Steps,
		StartTime:    adapterResult.StartTime,
		EndTime:      adapterResult.EndTime,
		Duration:     adapterResult.Duration,
		Output:       adapterResult.Output,
		ResultJSON:   adapterResult.ResultJSON,
		ErrorMessage: adapterResult.ErrorMessage,
	}

	e.log.Info("Task execution completed",
		"task_id", taskID,
		"status", execResult.Status,
		"duration", execResult.Duration,
	)

	return execResult, nil
}

func (e *Engine) ExecuteBatch(ctx context.Context, tasks []*adapter.Task) ([]*ExecutionResult, error) {
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks to execute")
	}

	executionID := generateID()
	status := &ExecutionStatus{
		ExecutionID: executionID,
		Status:      StatusRunning,
		Tasks:       tasks,
		Results:     make([]*ExecutionResult, len(tasks)),
		StartTime:   time.Now(),
	}

	e.mu.Lock()
	e.tasks[executionID] = status
	e.mu.Unlock()

	defer func() {
		status.EndTime = time.Now()
		if status.Status == StatusRunning {
			status.Status = StatusSuccess
		}
	}()

	maxParallel := e.cfg.Execution.MaxParallel
	if maxParallel < 1 {
		maxParallel = 1
	}

	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstError error

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t *adapter.Task) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			execResult, err := e.Execute(ctx, t)
			if err != nil {
				mu.Lock()
				if firstError == nil {
					firstError = err
				}
				execResult = &ExecutionResult{
					TaskID:       t.ID,
					Status:       StatusFailure,
					ErrorMessage: err.Error(),
				}
				mu.Unlock()
			}

			mu.Lock()
			status.Results[idx] = execResult
			if execResult.Status == StatusFailure {
				status.Status = StatusFailure
			}
			mu.Unlock()
		}(i, task)
	}

	wg.Wait()

	return status.Results, firstError
}

func (e *Engine) GetStatus(executionID string) (*ExecutionStatus, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	status, ok := e.tasks[executionID]
	if !ok {
		return nil, fmt.Errorf("execution not found: %s", executionID)
	}
	return status, nil
}

func (e *Engine) Cancel(executionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	status, ok := e.tasks[executionID]
	if !ok {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	if status.Status != StatusRunning {
		return fmt.Errorf("execution is not running: %s", executionID)
	}

	status.Status = StatusCancelled
	status.EndTime = time.Now()

	return nil
}

func convertStatus(s result.ScriptStatus) Status {
	switch s {
	case result.ScriptStatusSuccess:
		return StatusSuccess
	case result.ScriptStatusFailure:
		return StatusFailure
	case result.ScriptStatusWarning:
		return StatusFailure
	default:
		return StatusFailure
	}
}

func generateID() string {
	return fmt.Sprintf("exec-%d", time.Now().UnixNano())
}
