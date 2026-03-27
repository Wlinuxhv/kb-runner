package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"kb-runnerx/pkg/result"
)

type QResultManager struct {
	resultRoot string
}

type ExecutionEntry struct {
	DirName     string          `json:"dir_name"`
	QNo         string          `json:"qno"`
	Timestamp   string          `json:"timestamp"`
	ExecID      string          `json:"exec_id"`
	ExecTime    time.Time       `json:"exec_time"`
	ScriptCount int             `json:"script_count"`
	Summary     *HistorySummary `json:"summary"`
}

func NewQResultManager(resultRoot string) *QResultManager {
	path := resultRoot
	if strings.HasPrefix(resultRoot, "~/") {
		home := os.Getenv("HOME")
		if home != "" {
			path = filepath.Join(home, resultRoot[2:])
		}
	}
	return &QResultManager{
		resultRoot: path,
	}
}

func (m *QResultManager) ListExecutions() ([]*ExecutionEntry, error) {
	entries := make([]*ExecutionEntry, 0)

	files, err := os.ReadDir(m.resultRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return nil, err
	}

	pattern := regexp.MustCompile(`^(Q\d+)-(\d{14})$`)

	for _, f := range files {
		if !f.IsDir() {
			continue
		}

		dirName := f.Name()
		matches := pattern.FindStringSubmatch(dirName)
		if matches == nil {
			continue
		}

		qno := matches[1]
		timestamp := matches[2]

		dirPath := filepath.Join(m.resultRoot, dirName)
		info, err := f.Info()
		if err != nil {
			continue
		}

		execEntry, err := m.parseExecutionEntry(dirPath, dirName, qno, timestamp, info.ModTime())
		if err != nil {
			continue
		}

		entries = append(entries, execEntry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ExecTime.After(entries[j].ExecTime)
	})

	return entries, nil
}

func (m *QResultManager) parseExecutionEntry(dirPath, dirName, qno, timestamp string, modTime time.Time) (*ExecutionEntry, error) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var execID string
	var scriptCount int
	var summary *HistorySummary

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		if strings.HasPrefix(f.Name(), "ranked_results_") && strings.HasSuffix(f.Name(), ".json") {
			execID = strings.TrimSuffix(strings.TrimPrefix(f.Name(), "ranked_results_"), ".json")

			filePath := filepath.Join(dirPath, f.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			var result struct {
				Scripts []*result.ScriptScore `json:"scripts"`
				Summary struct {
					TotalScripts    int     `json:"total_scripts"`
					SuccessCount    int     `json:"success_count"`
					FailureCount    int     `json:"failure_count"`
					WarningCount    int     `json:"warning_count"`
					AverageScore    float64 `json:"average_score"`
					WeightedAverage float64 `json:"weighted_average"`
				} `json:"summary"`
			}

			if err := json.Unmarshal(data, &result); err == nil {
				scriptCount = len(result.Scripts)
				summary = &HistorySummary{
					TotalScripts:    result.Summary.TotalScripts,
					SuccessCount:    result.Summary.SuccessCount,
					FailureCount:    result.Summary.FailureCount,
					WarningCount:    result.Summary.WarningCount,
					AverageScore:    result.Summary.AverageScore,
					WeightedAverage: result.Summary.WeightedAverage,
				}
			}
		}
	}

	if execID == "" {
		execID = timestamp
	}

	execTime, err := time.Parse("20060102150405", timestamp)
	if err != nil {
		execTime = modTime
	}

	return &ExecutionEntry{
		DirName:     dirName,
		QNo:         qno,
		Timestamp:   timestamp,
		ExecID:      execID,
		ExecTime:    execTime,
		ScriptCount: scriptCount,
		Summary:     summary,
	}, nil
}

func (m *QResultManager) GetExecution(dirName string) (*result.ResultMatrix, error) {
	dirPath := filepath.Join(m.resultRoot, dirName)

	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		if strings.HasPrefix(f.Name(), "ranked_results_") && strings.HasSuffix(f.Name(), ".json") {
			filePath := filepath.Join(dirPath, f.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				return nil, err
			}

			var matrixResult result.ResultMatrix
			if err := json.Unmarshal(data, &matrixResult); err != nil {
				return nil, err
			}

			return &matrixResult, nil
		}
	}

	return nil, fmt.Errorf("no ranked_results file found in %s", dirName)
}

func (m *QResultManager) DeleteExecution(dirName string) error {
	dirPath := filepath.Join(m.resultRoot, dirName)
	return os.RemoveAll(dirPath)
}

func (m *QResultManager) ListQNos() ([]string, error) {
	qnoMap := make(map[string]bool)

	files, err := os.ReadDir(m.resultRoot)
	if err != nil {
		return nil, err
	}

	pattern := regexp.MustCompile(`^(Q\d+)-\d{14}$`)

	for _, f := range files {
		if !f.IsDir() {
			continue
		}

		matches := pattern.FindStringSubmatch(f.Name())
		if matches != nil {
			qnoMap[matches[1]] = true
		}
	}

	qnos := make([]string, 0, len(qnoMap))
	for qno := range qnoMap {
		qnos = append(qnos, qno)
	}

	sort.Strings(qnos)
	return qnos, nil
}

func (m *QResultManager) GetExecutionsByQNo(qno string) ([]*ExecutionEntry, error) {
	allEntries, err := m.ListExecutions()
	if err != nil {
		return nil, err
	}

	entries := make([]*ExecutionEntry, 0)
	for _, e := range allEntries {
		if e.QNo == qno {
			entries = append(entries, e)
		}
	}

	return entries, nil
}

func (m *QResultManager) DeleteQNo(qno string) error {
	entries, err := m.GetExecutionsByQNo(qno)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if err := m.DeleteExecution(e.DirName); err != nil {
			return err
		}
	}

	return nil
}

func (m *QResultManager) DeleteAll() error {
	files, err := os.ReadDir(m.resultRoot)
	if err != nil {
		return err
	}

	pattern := regexp.MustCompile(`^Q\d+-\d{14}$`)

	for _, f := range files {
		if !f.IsDir() {
			continue
		}

		if pattern.MatchString(f.Name()) {
			dirPath := filepath.Join(m.resultRoot, f.Name())
			if err := os.RemoveAll(dirPath); err != nil {
				return err
			}
		}
	}

	return nil
}
