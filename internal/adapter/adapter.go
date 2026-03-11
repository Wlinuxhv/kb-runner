package adapter

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kb-runnerx/pkg/result"
)

type Language string

const (
	LanguageBash   Language = "bash"
	LanguagePython Language = "python"
)

type Task struct {
	ID           string
	ScriptPath   string
	Language     Language
	Params       map[string]string
	Timeout      time.Duration
	WorkDir      string
	Env          map[string]string
	Weight       float64
	ScriptName   string
}

type ExecutionResult struct {
	TaskID       string
	Status       result.ScriptStatus
	Steps        []*result.StepResult
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
	Output       string
	ResultJSON   string
	ErrorMessage string
}

type Adapter interface {
	Language() Language
	Execute(ctx context.Context, task *Task) (*ExecutionResult, error)
	Validate(scriptPath string) error
	PrepareEnvironment(task *Task) error
}

type Registry struct {
	adapters map[Language]Adapter
	mu       sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[Language]Adapter),
	}
}

func (r *Registry) Register(language Language, adapter Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[language] = adapter
}

func (r *Registry) Get(language Language) (Adapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapter, ok := r.adapters[language]
	if !ok {
		return nil, fmt.Errorf("adapter not found for language: %s", language)
	}
	return adapter, nil
}

func (r *Registry) Languages() []Language {
	r.mu.RLock()
	defer r.mu.RUnlock()

	languages := make([]Language, 0, len(r.adapters))
	for lang := range r.adapters {
		languages = append(languages, lang)
	}
	return languages
}

type BashAdapter struct {
	apiScriptPath string
	tempDir       string
}

func NewBashAdapter(apiScriptPath, tempDir string) *BashAdapter {
	return &BashAdapter{
		apiScriptPath: apiScriptPath,
		tempDir:       tempDir,
	}
}

func (a *BashAdapter) Language() Language {
	return LanguageBash
}

func (a *BashAdapter) Validate(scriptPath string) error {
	info, err := os.Stat(scriptPath)
	if err != nil {
		return fmt.Errorf("script file not found: %s", scriptPath)
	}

	if info.IsDir() {
		return fmt.Errorf("script path is a directory: %s", scriptPath)
	}

	if info.Mode()&0111 == 0 {
		return fmt.Errorf("script is not executable: %s", scriptPath)
	}

	return nil
}

func (a *BashAdapter) PrepareEnvironment(task *Task) error {
	if task.WorkDir == "" {
		task.WorkDir = a.tempDir
	}

	if err := os.MkdirAll(task.WorkDir, 0755); err != nil {
		return fmt.Errorf("failed to create work directory: %w", err)
	}

	return nil
}

func (a *BashAdapter) Execute(ctx context.Context, task *Task) (*ExecutionResult, error) {
	execResult := &ExecutionResult{
		TaskID:    task.ID,
		Status:    result.ScriptStatusRunning,
		StartTime: time.Now(),
		Steps:     make([]*result.StepResult, 0),
	}

	tempResultFile := filepath.Join(a.tempDir, fmt.Sprintf("%s_result.json", task.ID))
	tempLogFile := filepath.Join(a.tempDir, fmt.Sprintf("%s.log", task.ID))

	env := a.buildEnv(task, tempResultFile, tempLogFile)

	cmd := exec.CommandContext(ctx, "bash", "-c", 
		fmt.Sprintf("source %s && source %s", a.apiScriptPath, task.ScriptPath))
	cmd.Dir = task.WorkDir
	cmd.Env = append(os.Environ(), env...)

	output, err := cmd.CombinedOutput()
	execResult.Output = string(output)
	execResult.EndTime = time.Now()
	execResult.Duration = execResult.EndTime.Sub(execResult.StartTime)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			execResult.Status = result.ScriptStatusFailure
			execResult.ErrorMessage = "execution timeout"
		} else {
			execResult.Status = result.ScriptStatusFailure
			execResult.ErrorMessage = fmt.Sprintf("execution failed: %v", err)
		}
	} else {
		execResult.Status = result.ScriptStatusSuccess
	}

	if data, err := os.ReadFile(tempResultFile); err == nil {
		execResult.ResultJSON = string(data)
	}

	defer os.Remove(tempResultFile)
	defer os.Remove(tempLogFile)

	return execResult, nil
}

func (a *BashAdapter) buildEnv(task *Task, resultFile, logFile string) []string {
	env := []string{
		fmt.Sprintf("KB_RESULT_FILE=%s", resultFile),
		fmt.Sprintf("KB_LOG_FILE=%s", logFile),
		fmt.Sprintf("KB_SCRIPT_NAME=%s", task.ScriptName),
		"KB_LOG_LEVEL=INFO",
	}

	for k, v := range task.Params {
		env = append(env, fmt.Sprintf("KB_PARAM_%s=%s", strings.ToUpper(k), v))
	}

	for k, v := range task.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}

type PythonAdapter struct {
	apiScriptPath string
	tempDir       string
	pythonPath    string
}

func NewPythonAdapter(apiScriptPath, tempDir, pythonPath string) *PythonAdapter {
	if pythonPath == "" {
		pythonPath = "python3"
	}
	return &PythonAdapter{
		apiScriptPath: apiScriptPath,
		tempDir:       tempDir,
		pythonPath:    pythonPath,
	}
}

func (a *PythonAdapter) Language() Language {
	return LanguagePython
}

func (a *PythonAdapter) Validate(scriptPath string) error {
	info, err := os.Stat(scriptPath)
	if err != nil {
		return fmt.Errorf("script file not found: %s", scriptPath)
	}

	if info.IsDir() {
		return fmt.Errorf("script path is a directory: %s", scriptPath)
	}

	return nil
}

func (a *PythonAdapter) PrepareEnvironment(task *Task) error {
	if task.WorkDir == "" {
		task.WorkDir = a.tempDir
	}

	if err := os.MkdirAll(task.WorkDir, 0755); err != nil {
		return fmt.Errorf("failed to create work directory: %w", err)
	}

	return nil
}

func (a *PythonAdapter) Execute(ctx context.Context, task *Task) (*ExecutionResult, error) {
	execResult := &ExecutionResult{
		TaskID:    task.ID,
		Status:    result.ScriptStatusRunning,
		StartTime: time.Now(),
		Steps:     make([]*result.StepResult, 0),
	}

	tempResultFile := filepath.Join(a.tempDir, fmt.Sprintf("%s_result.json", task.ID))
	tempLogFile := filepath.Join(a.tempDir, fmt.Sprintf("%s.log", task.ID))

	env := a.buildEnv(task, tempResultFile, tempLogFile)

	scriptContent := fmt.Sprintf(`
import sys
sys.path.insert(0, '%s')
from kb_api import *
exec(open('%s').read())
`, a.apiScriptPath, task.ScriptPath)

	cmd := exec.CommandContext(ctx, a.pythonPath, "-c", scriptContent)
	cmd.Dir = task.WorkDir
	cmd.Env = append(os.Environ(), env...)

	output, err := cmd.CombinedOutput()
	execResult.Output = string(output)
	execResult.EndTime = time.Now()
	execResult.Duration = execResult.EndTime.Sub(execResult.StartTime)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			execResult.Status = result.ScriptStatusFailure
			execResult.ErrorMessage = "execution timeout"
		} else {
			execResult.Status = result.ScriptStatusFailure
			execResult.ErrorMessage = fmt.Sprintf("execution failed: %v", err)
		}
	} else {
		execResult.Status = result.ScriptStatusSuccess
	}

	if data, err := os.ReadFile(tempResultFile); err == nil {
		execResult.ResultJSON = string(data)
	}

	defer os.Remove(tempResultFile)
	defer os.Remove(tempLogFile)

	return execResult, nil
}

func (a *PythonAdapter) buildEnv(task *Task, resultFile, logFile string) []string {
	env := []string{
		fmt.Sprintf("KB_RESULT_FILE=%s", resultFile),
		fmt.Sprintf("KB_LOG_FILE=%s", logFile),
		fmt.Sprintf("KB_SCRIPT_NAME=%s", task.ScriptName),
		"KB_LOG_LEVEL=INFO",
	}

	for k, v := range task.Params {
		env = append(env, fmt.Sprintf("KB_PARAM_%s=%s", strings.ToUpper(k), v))
	}

	for k, v := range task.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}
