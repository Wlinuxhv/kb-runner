package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"kb-runnerx/pkg/config"
	"kb-runnerx/pkg/result"
)

type HistoryManager struct {
	historyDir      string
	maxRecords      int
	autoCleanup     bool
	cleanupThreshold float64
	mu              sync.RWMutex
}

type HistoryRecord struct {
	ExecutionID string              `json:"execution_id"`
	Timestamp   string              `json:"timestamp"`
	Status      string              `json:"status"`
	Trigger     string              `json:"trigger"`
	Scenario    string              `json:"scenario,omitempty"`
	Cases       []string            `json:"cases"`
	Summary     *HistorySummary     `json:"summary"`
	Result      *result.ResultMatrix `json:"result"`
}

type HistorySummary struct {
	TotalScripts    int     `json:"total_scripts"`
	SuccessCount    int     `json:"success_count"`
	FailureCount    int     `json:"failure_count"`
	WarningCount    int     `json:"warning_count"`
	AverageScore    float64 `json:"average_score"`
	WeightedAverage float64 `json:"weighted_average"`
}

func NewHistoryManager(cfg *config.Config) *HistoryManager {
	historyDir := filepath.Join(cfg.Execution.WorkDir, "history")
	os.MkdirAll(historyDir, 0755)

	maxRecords := cfg.History.MaxRecords
	if maxRecords <= 0 {
		maxRecords = 4294967296
	}

	return &HistoryManager{
		historyDir:       historyDir,
		maxRecords:       maxRecords,
		autoCleanup:      cfg.History.AutoCleanup,
		cleanupThreshold: cfg.History.CleanupThreshold,
	}
}

func (h *HistoryManager) Save(record *HistoryRecord) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	filename := fmt.Sprintf("history_%s.json", record.ExecutionID)
	path := filepath.Join(h.historyDir, filename)

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	if h.autoCleanup {
		h.checkAndCleanup()
	}

	return nil
}

func (h *HistoryManager) checkAndCleanup() {
	files, _ := os.ReadDir(h.historyDir)
	count := len(files)

	threshold := int(float64(h.maxRecords) * h.cleanupThreshold)
	if count >= threshold {
		h.cleanupOldest(count - h.maxRecords + 100)
	}
}

func (h *HistoryManager) cleanupOldest(toDelete int) {
	files, _ := os.ReadDir(h.historyDir)

	sort.Slice(files, func(i, j int) bool {
		info1, _ := files[i].Info()
		info2, _ := files[j].Info()
		return info1.ModTime().Before(info2.ModTime())
	})

	for i := 0; i < toDelete && i < len(files); i++ {
		path := filepath.Join(h.historyDir, files[i].Name())
		os.Remove(path)
	}
}

func (h *HistoryManager) List(limit int) ([]*HistoryRecord, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	files, err := os.ReadDir(h.historyDir)
	if err != nil {
		return nil, err
	}

	var records []*HistoryRecord
	for _, f := range files {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}

		path := filepath.Join(h.historyDir, f.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var record HistoryRecord
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}

		records = append(records, &record)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp > records[j].Timestamp
	})

	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}

	return records, nil
}

func (h *HistoryManager) Get(executionID string) (*HistoryRecord, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	filename := fmt.Sprintf("history_%s.json", executionID)
	path := filepath.Join(h.historyDir, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var record HistoryRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}

	return &record, nil
}

func (h *HistoryManager) Delete(executionID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	filename := fmt.Sprintf("history_%s.json", executionID)
	path := filepath.Join(h.historyDir, filename)

	return os.Remove(path)
}

func (h *HistoryManager) ClearAll() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	files, err := os.ReadDir(h.historyDir)
	if err != nil {
		return err
	}

	for _, f := range files {
		path := filepath.Join(h.historyDir, f.Name())
		os.Remove(path)
	}

	return nil
}

func (h *HistoryManager) Export(executionID, format string) ([]byte, error) {
	record, err := h.Get(executionID)
	if err != nil {
		return nil, err
	}

	switch format {
	case "yaml":
		return yamlMarshal(record)
	default:
		return json.MarshalIndent(record, "", "  ")
	}
}

func yamlMarshal(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	
	return jsonToYAML(obj), nil
}

func jsonToYAML(obj map[string]interface{}) []byte {
	var result []byte
	yamlEncode(obj, "", &result)
	return result
}

func yamlEncode(obj map[string]interface{}, indent string, result *[]byte) {
	for k, v := range obj {
		switch val := v.(type) {
		case map[string]interface{}:
			*result = append(*result, fmt.Sprintf("%s%s:\n", indent, k)...)
			yamlEncode(val, indent+"  ", result)
		case []interface{}:
			*result = append(*result, fmt.Sprintf("%s%s:\n", indent, k)...)
			for _, item := range val {
				if m, ok := item.(map[string]interface{}); ok {
					*result = append(*result, fmt.Sprintf("%s  -\n", indent)...)
					for mk, mv := range m {
						*result = append(*result, fmt.Sprintf("%s    %s: %v\n", indent, mk, formatValue(mv))...)
					}
				} else {
					*result = append(*result, fmt.Sprintf("%s  - %v\n", indent, formatValue(item))...)
				}
			}
		default:
			*result = append(*result, fmt.Sprintf("%s%s: %v\n", indent, k, formatValue(val))...)
		}
	}
}

func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%d", int(val))
		}
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}