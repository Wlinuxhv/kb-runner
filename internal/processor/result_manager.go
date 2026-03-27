package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ResultManager 管理结果文件的保存和清理
type ResultManager struct {
	resultRoot string
	maxRecords int
	keepDays   int
}

// NewResultManager 创建结果管理器
func NewResultManager(resultRoot string, maxRecords int) *ResultManager {
	return &ResultManager{
		resultRoot: resultRoot,
		maxRecords: maxRecords,
	}
}

// SaveResult 保存结果文件
func (m *ResultManager) SaveResult(qno string, filename string, data []byte) error {
	// 创建 Q 单号目录
	qnoDir := filepath.Join(m.resultRoot, qno)
	if err := os.MkdirAll(qnoDir, 0755); err != nil {
		return fmt.Errorf("failed to create qno directory: %w", err)
	}

	// 保存文件
	filepath := filepath.Join(qnoDir, filename)
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write result file: %w", err)
	}

	return nil
}

// CleanupOldResults 清理旧的结果文件
func (m *ResultManager) CleanupOldResults(qno string) error {
	qnoDir := filepath.Join(m.resultRoot, qno)
	if _, err := os.Stat(qnoDir); os.IsNotExist(err) {
		return nil // 目录不存在，无需清理
	}

	// 读取所有结果文件
	entries, err := os.ReadDir(qnoDir)
	if err != nil {
		return fmt.Errorf("failed to read qno directory: %w", err)
	}

	// 收集所有 ranked_results 文件及其时间
	type resultFile struct {
		path      string
		timestamp time.Time
	}

	var resultFiles []resultFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, "ranked_results_") || !strings.HasSuffix(name, ".json") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		resultFiles = append(resultFiles, resultFile{
			path:      filepath.Join(qnoDir, name),
			timestamp: info.ModTime(),
		})
	}

	// 按时间排序（最新的在前）
	sort.Slice(resultFiles, func(i, j int) bool {
		return resultFiles[i].timestamp.After(resultFiles[j].timestamp)
	})

	// 保留最新的 N 条记录
	if len(resultFiles) > m.maxRecords {
		for i := m.maxRecords; i < len(resultFiles); i++ {
			// 删除旧文件
			if err := os.Remove(resultFiles[i].path); err != nil {
				return fmt.Errorf("failed to remove old result file: %w", err)
			}

			// 同时删除对应的单个 KB 结果文件
			// 从 ranked_results_<exec_id>.json 提取 exec_id
			execID := strings.TrimSuffix(strings.TrimPrefix(resultFiles[i].path, "ranked_results_"), ".json")

			// 查找并删除对应的单个 KB 结果
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				if strings.HasSuffix(name, "_"+execID+"_result.json") {
					os.Remove(filepath.Join(qnoDir, name))
				}
			}
		}
	}

	return nil
}

// GetResultFiles 获取 Q 单号下的所有结果文件
func (m *ResultManager) GetResultFiles(qno string) ([]string, error) {
	qnoDir := filepath.Join(m.resultRoot, qno)
	if _, err := os.Stat(qnoDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(qnoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read qno directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, entry.Name())
	}

	return files, nil
}

// GetLatestResult 获取最新的结果文件
func (m *ResultManager) GetLatestResult(qno string) (string, error) {
	files, err := m.GetResultFiles(qno)
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", nil
	}

	// 查找最新的 ranked_results 文件
	var latestFile string
	var latestTime time.Time

	for _, file := range files {
		if !strings.HasPrefix(file, "ranked_results_") {
			continue
		}

		info, err := os.Stat(filepath.Join(m.resultRoot, qno, file))
		if err != nil {
			continue
		}

		if latestFile == "" || info.ModTime().After(latestTime) {
			latestFile = file
			latestTime = info.ModTime()
		}
	}

	return latestFile, nil
}
