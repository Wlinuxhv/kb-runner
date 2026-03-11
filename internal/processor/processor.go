package processor

import (
	"encoding/json"
	"fmt"
	"sync"

	"kb-runnerx/internal/executor"
	"kb-runnerx/pkg/config"
	"kb-runnerx/pkg/logger"
	"kb-runnerx/pkg/result"
)

type Processor struct {
	cfg *config.Config
	log *logger.Logger
}

func NewProcessor(cfg *config.Config, log *logger.Logger) *Processor {
	return &Processor{
		cfg: cfg,
		log: log,
	}
}

func (p *Processor) Parse(output string) (*result.ScriptResult, error) {
	if output == "" {
		return nil, fmt.Errorf("empty output")
	}

	var scriptResult result.ScriptResult
	if err := json.Unmarshal([]byte(output), &scriptResult); err != nil {
		return nil, fmt.Errorf("failed to parse output: %w", err)
	}

	return &scriptResult, nil
}

func (p *Processor) CalculateWeight(scriptResult *result.ScriptResult, scriptName string) float64 {
	weight := p.cfg.GetScriptWeight(scriptName)
	return scriptResult.Score * weight
}

func (p *Processor) Normalize(scores []float64) []float64 {
	if len(scores) == 0 {
		return scores
	}

	min := scores[0]
	max := scores[0]
	for _, s := range scores {
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
	}

	if max == min {
		normalized := make([]float64, len(scores))
		for i := range normalized {
			normalized[i] = 0.5
		}
		return normalized
	}

	normalized := make([]float64, len(scores))
	for i, s := range scores {
		normalized[i] = (s - min) / (max - min)
	}

	return normalized
}

func (p *Processor) Process(execResults []*executor.ExecutionResult) (*result.ResultMatrix, error) {
	if len(execResults) == 0 {
		return nil, fmt.Errorf("no results to process")
	}

	matrix := result.NewResultMatrix(generateExecutionID())

	var scores []float64
	for _, execResult := range execResults {
		scriptName := execResult.TaskID
		if execResult.TaskID == "" {
			scriptName = "unknown"
		}

		score := execResult.Score
		if score == 0 && execResult.ResultJSON != "" {
			var scriptResult result.ScriptResult
			if err := json.Unmarshal([]byte(execResult.ResultJSON), &scriptResult); err == nil {
				score = scriptResult.Score
			}
		}

		weight := p.cfg.GetScriptWeight(scriptName)
		weightedScore := score * weight

		matrix.AddScript(scriptName, score, weight, string(execResult.Status))
		scores = append(scores, weightedScore)
	}

	matrix.Calculate()

	p.log.Info("Result matrix generated",
		"total_scripts", matrix.Summary.TotalScripts,
		"success_count", matrix.Summary.SuccessCount,
		"failure_count", matrix.Summary.FailureCount,
		"average_score", matrix.Summary.AverageScore,
	)

	return matrix, nil
}

func (p *Processor) ProcessBatch(execResults []*executor.ExecutionResult) ([]*result.ScriptResult, *result.ResultMatrix, error) {
	scriptResults := make([]*result.ScriptResult, 0, len(execResults))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, execResult := range execResults {
		wg.Add(1)
		go func(er *executor.ExecutionResult) {
			defer wg.Done()

			if er.ResultJSON == "" {
				return
			}

			var sr result.ScriptResult
			if err := json.Unmarshal([]byte(er.ResultJSON), &sr); err != nil {
				p.log.Warn("Failed to parse script result",
					"task_id", er.TaskID,
					"error", err,
				)
				return
			}

			mu.Lock()
			scriptResults = append(scriptResults, &sr)
			mu.Unlock()
		}(execResult)
	}

	wg.Wait()

	matrix, err := p.Process(execResults)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate matrix: %w", err)
	}

	return scriptResults, matrix, nil
}

func (p *Processor) GenerateReport(matrix *result.ResultMatrix) (string, error) {
	data, err := matrix.ToJSON()
	if err != nil {
		return "", fmt.Errorf("failed to generate report: %w", err)
	}
	return string(data), nil
}

func generateExecutionID() string {
	return fmt.Sprintf("exec-%d", currentTime().UnixNano())
}

type TimeFunc func() interface{ UnixNano() int64 }

func currentTime() interface{ UnixNano() int64 } {
	return timeNow{}
}

type timeNow struct{}

func (timeNow) UnixNano() int64 {
	return 0
}
