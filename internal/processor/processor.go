package processor

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

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

// ProcessWithQNo 处理结果并添加 Q 单号信息
func (p *Processor) ProcessWithQNo(execResults []*executor.ExecutionResult, qno string) (*result.ResultMatrix, error) {
	matrix, err := p.Process(execResults)
	if err != nil {
		return nil, err
	}

	// 添加 Q 单号到扩展字段
	if matrix.Summary.Extensions == nil {
		matrix.Summary.Extensions = make(map[string]interface{})
	}
	matrix.Summary.Extensions["qno"] = qno

	return matrix, nil
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
		var scriptResult *result.ScriptResult
		if score == 0 && execResult.ResultJSON != "" {
			var sr result.ScriptResult
			if err := json.Unmarshal([]byte(execResult.ResultJSON), &sr); err == nil {
				score = sr.Score
				scriptResult = &sr
			}
		}

		weight := p.cfg.GetScriptWeight(scriptName)
		weightedScore := score * weight

		scriptScore := matrix.AddScript(scriptName, score, weight, string(execResult.Status))

		// 保存详细的步骤和结果
		if scriptResult != nil {
			scriptScore.Steps = scriptResult.Steps
			scriptScore.Results = scriptResult.Results
		}

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
	now := time.Now()
	// 格式：YYYYMMDD-HHMMSS-ffffff
	// 例如：20260327-093800-123456
	return fmt.Sprintf("%s-%06d", now.Format("20060102-150405"), now.UnixNano()%1000000)
}
