package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"kb-runnerx/internal/cases"
)

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, Response{
		Success:   false,
		Error:     message,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// SkillManager 处理 Skill.md 文件管理
type SkillManager struct {
	basePath string // CASE的基础路径
}

// NewSkillManager 创建Skill管理器
func NewSkillManager(basePath string) *SkillManager {
	return &SkillManager{
		basePath: basePath,
	}
}

// GetSkill 获取指定CASE的Skill.md内容
func (sm *SkillManager) GetSkill(caseName string) (string, error) {
	skillPath := sm.getSkillPath(caseName)
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return "", fmt.Errorf("skill file not found: %w", err)
	}
	return string(data), nil
}

// UpdateSkill 更新指定CASE的Skill.md内容
func (sm *SkillManager) UpdateSkill(caseName, content string) error {
	skillPath := sm.getSkillPath(caseName)

	// 确保目录存在
	dir := filepath.Dir(skillPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write skill file: %w", err)
	}

	return nil
}

// GetSkillPath 获取Skill.md文件的完整路径
func (sm *SkillManager) getSkillPath(caseName string) string {
	// 首先检查workspace/cases目录
	path := filepath.Join(sm.basePath, "cases", caseName, "Skill.md")
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// 检查中台目录 (可能与workspace同级)
	path = filepath.Join(sm.basePath, "中台", caseName, "Skill.md")
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// 检查父目录的中台
	path = filepath.Join(filepath.Dir(sm.basePath), "中台", caseName, "Skill.md")
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// 回退到workspace路径
	return filepath.Join(sm.basePath, "cases", caseName, "Skill.md")
}

// SkillVersion Skill版本历史
type SkillVersion struct {
	KBCode    string    `json:"kb_code"`
	Version   int64     `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content,omitempty"`
}

// GetHistory 获取Skill版本历史列表
func (sm *SkillManager) GetHistory(caseName string) ([]SkillVersion, error) {
	historyDir := sm.getHistoryDir(caseName)

	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SkillVersion{}, nil
		}
		return nil, err
	}

	versions := make([]SkillVersion, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".md" {
			continue
		}

		version, err := strconv.ParseInt(strings.TrimSuffix(entry.Name(), ".md"), 10, 64)
		if err != nil {
			continue
		}

		versions = append(versions, SkillVersion{
			KBCode:    caseName,
			Version:   version,
			Timestamp: time.Unix(version, 0),
		})
	}

	// 按时间倒序排列
	for i := 0; i < len(versions)-1; i++ {
		for j := i + 1; j < len(versions); j++ {
			if versions[i].Version < versions[j].Version {
				versions[i], versions[j] = versions[j], versions[i]
			}
		}
	}

	return versions, nil
}

// GetVersion 获取指定版本的Skill内容
func (sm *SkillManager) GetVersion(caseName string, version int64) (string, error) {
	historyDir := sm.getHistoryDir(caseName)
	versionPath := filepath.Join(historyDir, fmt.Sprintf("%d.md", version))

	data, err := os.ReadFile(versionPath)
	if err != nil {
		return "", fmt.Errorf("version not found: %w", err)
	}

	return string(data), nil
}

// Rollback 回滚到指定版本
func (sm *SkillManager) Rollback(caseName string, version int64) error {
	// 获取指定版本的内容
	content, err := sm.GetVersion(caseName, version)
	if err != nil {
		return err
	}

	// 更新当前文件
	if err := sm.UpdateSkill(caseName, content); err != nil {
		return err
	}

	// 创建新版本记录
	return sm.saveVersion(caseName, content)
}

// SaveVersion 保存当前版本到历史
func (sm *SkillManager) SaveVersion(caseName, content string) error {
	return sm.saveVersion(caseName, content)
}

func (sm *SkillManager) saveVersion(caseName, content string) error {
	historyDir := sm.getHistoryDir(caseName)
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	version := time.Now().Unix()
	versionPath := filepath.Join(historyDir, fmt.Sprintf("%d.md", version))

	return os.WriteFile(versionPath, []byte(content), 0644)
}

func (sm *SkillManager) getHistoryDir(caseName string) string {
	// 历史版本保存在workspace/skill_history目录
	return filepath.Join(sm.basePath, "skill_history", caseName)
}

// GetConfig 获取CASE的配置信息
func (sm *SkillManager) GetConfig(caseName string, caseMgr *cases.Manager) (map[string]interface{}, error) {
	// 从cases管理器获取配置
	c, err := caseMgr.Get(caseName)
	if err != nil {
		return nil, fmt.Errorf("case not found: %w", err)
	}

	config := map[string]interface{}{
		"kb_code":          caseName,
		"weight":           c.Weight,
		"category":         c.Category,
		"tags":             c.Tags,
		"skill_path":       sm.getSkillPath(caseName),
		"analysis_objects": c.Params,
	}

	return config, nil
}

// HandleSkillGet 获取Skill内容
func (sm *SkillManager) HandleSkillGet(w http.ResponseWriter, r *http.Request) {
	caseName := strings.TrimPrefix(r.URL.Path, "/api/v1/kb/")
	caseName = strings.TrimSuffix(caseName, "/skill")

	if caseName == "" {
		writeError(w, http.StatusBadRequest, "case name required")
		return
	}

	content, err := sm.GetSkill(caseName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"kb_code":   caseName,
			"content":   content,
			"updated_at": time.Now().Format(time.RFC3339),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// HandleSkillUpdate 更新Skill内容
func (sm *SkillManager) HandleSkillUpdate(w http.ResponseWriter, r *http.Request) {
	// 检查权限
	if !IsAdmin(r) {
		writeError(w, http.StatusForbidden, "permission denied, admin only")
		return
	}

	caseName := strings.TrimPrefix(r.URL.Path, "/api/v1/kb/")
	caseName = strings.TrimSuffix(caseName, "/skill")

	if caseName == "" {
		writeError(w, http.StatusBadRequest, "case name required")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// 保存到历史版本
	if err := sm.SaveVersion(caseName, req.Content); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save version: "+err.Error())
		return
	}

	// 更新当前文件
	if err := sm.UpdateSkill(caseName, req.Content); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"message": "Skill 更新成功",
			"version": time.Now().Unix(),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// HandleSkillHistory 获取版本历史
func (sm *SkillManager) HandleSkillHistory(w http.ResponseWriter, r *http.Request) {
	// 检查权限
	if !IsAdmin(r) {
		writeError(w, http.StatusForbidden, "permission denied, admin only")
		return
	}

	caseName := strings.TrimPrefix(r.URL.Path, "/api/v1/kb/")
	caseName = strings.TrimSuffix(caseName, "/skill/history")

	if caseName == "" {
		writeError(w, http.StatusBadRequest, "case name required")
		return
	}

	versions, err := sm.GetHistory(caseName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"kb_code":  caseName,
			"versions": versions,
		},
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// HandleSkillHistoryDetail 获取指定版本内容
func (sm *SkillManager) HandleSkillHistoryDetail(w http.ResponseWriter, r *http.Request) {
	// 检查权限
	if !IsAdmin(r) {
		writeError(w, http.StatusForbidden, "permission denied, admin only")
		return
	}

	// 提取caseName和version
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/kb/")
	parts := strings.Split(path, "/skill/history/")

	if len(parts) != 2 {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	caseName := parts[0]
	versionStr := parts[1]
	version, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid version")
		return
	}

	content, err := sm.GetVersion(caseName, version)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"kb_code":  caseName,
			"version":  version,
			"content":  content,
			"timestamp": time.Unix(version, 0).Format(time.RFC3339),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// HandleSkillRollback 回滚到指定版本
func (sm *SkillManager) HandleSkillRollback(w http.ResponseWriter, r *http.Request) {
	// 检查权限
	if !IsAdmin(r) {
		writeError(w, http.StatusForbidden, "permission denied, admin only")
		return
	}

	caseName := strings.TrimPrefix(r.URL.Path, "/api/v1/kb/")
	caseName = strings.TrimSuffix(caseName, "/skill/rollback")

	if caseName == "" {
		writeError(w, http.StatusBadRequest, "case name required")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var req struct {
		Version int64 `json:"version"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := sm.Rollback(caseName, req.Version); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"message": fmt.Sprintf("已回滚到版本 %d", req.Version),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// HandleKBConfig 获取KB配置
func (sm *SkillManager) HandleKBConfig(w http.ResponseWriter, r *http.Request, caseMgr *cases.Manager) {
	caseName := strings.TrimPrefix(r.URL.Path, "/api/v1/kb/")

	if caseName == "" {
		writeError(w, http.StatusBadRequest, "case name required")
		return
	}

	config, err := sm.GetConfig(caseName, caseMgr)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success:   true,
		Data:      config,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}
