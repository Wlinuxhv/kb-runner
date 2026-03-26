package result

import (
	"encoding/json"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type StepStatus string

const (
	StepStatusSuccess StepStatus = "success"
	StepStatusFailure StepStatus = "failure"
	StepStatusWarning StepStatus = "warning"
	StepStatusSkipped StepStatus = "skipped"
)

type StepResult struct {
	Name           string                 `json:"name"`
	Status         StepStatus             `json:"status"`
	Message        string                 `json:"message,omitempty"`
	Output         string                 `json:"output,omitempty"`
	DurationMs     int64                  `json:"duration_ms"`
	StartTime      time.Time              `json:"start_time"`
	EndTime        time.Time              `json:"end_time"`
	Results        map[string]interface{} `json:"results,omitempty"`
	Weight         float64                `json:"weight,omitempty"`
	ExpectedStatus string                 `json:"expected_status,omitempty"`
	Score          float64                `json:"score,omitempty"`
	MaxScore       float64                `json:"max_score,omitempty"`
}

type ScriptStatus string

const (
	ScriptStatusRunning ScriptStatus = "running"
	ScriptStatusSuccess ScriptStatus = "success"
	ScriptStatusFailure ScriptStatus = "failure"
	ScriptStatusWarning ScriptStatus = "warning"
)

type ScriptResult struct {
	ScriptName string                 `json:"script_name"`
	Status     ScriptStatus           `json:"status"`
	Steps      []*StepResult          `json:"steps"`
	Results    map[string]interface{} `json:"results,omitempty"`
	Score      float64                `json:"score"`
	Message    string                 `json:"message,omitempty"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    time.Time              `json:"end_time"`
	DurationMs int64                  `json:"duration_ms"`
	mu         sync.Mutex
}

func NewScriptResult(scriptName string) *ScriptResult {
	return &ScriptResult{
		ScriptName: scriptName,
		Status:     ScriptStatusRunning,
		Steps:      make([]*StepResult, 0),
		Results:    make(map[string]interface{}),
		StartTime:  time.Now(),
	}
}

func (r *ScriptResult) StartStep(name string) *StepResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	step := &StepResult{
		Name:      name,
		Status:    StepStatusSuccess,
		StartTime: time.Now(),
		Results:   make(map[string]interface{}),
	}

	r.Steps = append(r.Steps, step)
	return step
}

func (r *ScriptResult) EndStep(step *StepResult, status StepStatus, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	step.Status = status
	step.Message = message
	step.EndTime = time.Now()
	step.DurationMs = step.EndTime.Sub(step.StartTime).Milliseconds()
}

func (r *ScriptResult) AddResult(key string, value interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Results[key] = value
}

func (r *ScriptResult) Finish(status ScriptStatus, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Status = status
	r.Message = message
	r.EndTime = time.Now()
	r.DurationMs = r.EndTime.Sub(r.StartTime).Milliseconds()

	r.Score = r.calculateScore()
}

func (r *ScriptResult) calculateScore() float64 {
	if len(r.Steps) == 0 {
		return 0
	}

	var totalScore float64
	var count int

	for _, step := range r.Steps {
		var stepScore float64
		switch step.Status {
		case StepStatusSuccess:
			stepScore = 1.0
		case StepStatusWarning:
			stepScore = 0.7
		case StepStatusSkipped:
			stepScore = 0.5
		case StepStatusFailure:
			stepScore = 0.0
		}
		totalScore += stepScore
		count++
	}

	if count == 0 {
		return 0
	}
	return totalScore / float64(count)
}

func (r *ScriptResult) ToJSON() ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return json.MarshalIndent(r, "", "  ")
}

func (r *ScriptResult) ToYAML() ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return yaml.Marshal(r)
}

type ResultMatrix struct {
	Timestamp     time.Time       `json:"timestamp"`
	ExecutionID   string          `json:"execution_id"`
	Scripts       []*ScriptScore  `json:"scripts"`
	Matrix        [][]float64     `json:"matrix"`
	Summary       MatrixSummary   `json:"summary"`
	RankedResults []*RankedResult `json:"ranked_results,omitempty"`
}

type ScriptScore struct {
	Name            string                 `json:"name"`
	RawScore        float64                `json:"raw_score"`
	WeightedScore   float64                `json:"weighted_score"`
	NormalizedScore float64                `json:"normalized_score"`
	Status          string                 `json:"status"`
	MaxScore        float64                `json:"max_score,omitempty"`
	FinalScore      float64                `json:"final_score"`
	Rank            int                    `json:"rank,omitempty"`
	Steps           []*StepResult          `json:"steps,omitempty"`
	Results         map[string]interface{} `json:"results,omitempty"`
	Extensions      map[string]interface{} `json:"extensions,omitempty"`
}

type MatrixSummary struct {
	TotalScripts     int     `json:"total_scripts"`
	SuccessCount     int     `json:"success_count"`
	FailureCount     int     `json:"failure_count"`
	WarningCount     int     `json:"warning_count"`
	AverageScore     float64 `json:"average_score"`
	WeightedAverage  float64 `json:"weighted_average"`
	MaxPossibleScore float64 `json:"max_possible_score,omitempty"`
}

type RankedResult struct {
	Rank       int     `json:"rank"`
	Name       string  `json:"name"`
	FinalScore float64 `json:"final_score"`
	Status     string  `json:"status"`
	MaxScore   float64 `json:"max_score,omitempty"`
}

func NewResultMatrix(executionID string) *ResultMatrix {
	return &ResultMatrix{
		Timestamp:   time.Now(),
		ExecutionID: executionID,
		Scripts:     make([]*ScriptScore, 0),
		Matrix:      make([][]float64, 0),
	}
}

func (m *ResultMatrix) AddScript(name string, score float64, weight float64, status string) *ScriptScore {
	script := &ScriptScore{
		Name:          name,
		RawScore:      score,
		WeightedScore: score * weight,
		Status:        status,
		Results:       make(map[string]interface{}),
		Extensions:    make(map[string]interface{}),
	}
	m.Scripts = append(m.Scripts, script)
	return script
}

func (m *ResultMatrix) Calculate() {
	if len(m.Scripts) == 0 {
		return
	}

	m.normalizeScores()
	m.generateMatrix()
	m.calculateSummary()
	m.calculateRanking()
}

func (m *ResultMatrix) normalizeScores() {
	if len(m.Scripts) == 0 {
		return
	}

	min := m.Scripts[0].WeightedScore
	max := m.Scripts[0].WeightedScore

	for _, s := range m.Scripts {
		if s.WeightedScore < min {
			min = s.WeightedScore
		}
		if s.WeightedScore > max {
			max = s.WeightedScore
		}
	}

	if max == min {
		for _, s := range m.Scripts {
			s.NormalizedScore = 0.5
		}
		return
	}

	for _, s := range m.Scripts {
		s.NormalizedScore = (s.WeightedScore - min) / (max - min)
	}
}

func (m *ResultMatrix) generateMatrix() {
	m.Matrix = make([][]float64, len(m.Scripts))
	for i, s := range m.Scripts {
		m.Matrix[i] = []float64{s.NormalizedScore}
	}
}

func (m *ResultMatrix) calculateSummary() {
	m.Summary.TotalScripts = len(m.Scripts)

	var totalRaw, totalWeighted, maxPossible float64
	for _, s := range m.Scripts {
		totalRaw += s.RawScore
		totalWeighted += s.WeightedScore
		if s.MaxScore > 0 {
			maxPossible = s.MaxScore
		}

		switch s.Status {
		case "success":
			m.Summary.SuccessCount++
		case "failure":
			m.Summary.FailureCount++
		case "warning":
			m.Summary.WarningCount++
		}
	}

	if m.Summary.TotalScripts > 0 {
		m.Summary.AverageScore = totalRaw / float64(m.Summary.TotalScripts)
		m.Summary.WeightedAverage = totalWeighted / float64(m.Summary.TotalScripts)
		m.Summary.MaxPossibleScore = maxPossible
	}
}

func (m *ResultMatrix) calculateRanking() {
	if len(m.Scripts) == 0 {
		return
	}

	sorted := make([]*ScriptScore, len(m.Scripts))
	copy(sorted, m.Scripts)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].FinalScore > sorted[i].FinalScore {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	m.RankedResults = make([]*RankedResult, len(sorted))
	for i, s := range sorted {
		s.Rank = i + 1
		m.RankedResults[i] = &RankedResult{
			Rank:       s.Rank,
			Name:       s.Name,
			FinalScore: s.FinalScore,
			Status:     s.Status,
			MaxScore:   s.MaxScore,
		}
	}

	scriptRankMap := make(map[string]int)
	for _, r := range m.RankedResults {
		scriptRankMap[r.Name] = r.Rank
	}
	for _, s := range m.Scripts {
		if rank, ok := scriptRankMap[s.Name]; ok {
			s.Rank = rank
		}
	}
}

func (m *ResultMatrix) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

func (m *ResultMatrix) ToYAML() ([]byte, error) {
	return yaml.Marshal(m)
}
