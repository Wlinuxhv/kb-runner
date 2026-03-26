package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kb-runnerx/internal/adapter"
	"kb-runnerx/internal/cases"
	"kb-runnerx/pkg/result"
)

type Response struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp string      `json:"timestamp"`
}

func (s *Server) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, statusCode int, message string) {
	s.writeJSON(w, statusCode, Response{
		Success:   false,
		Error:     message,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleCases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	category := r.URL.Query().Get("category")
	tags := r.URL.Query().Get("tags")

	filter := cases.FilterOptions{
		Category: category,
		Tags:     strings.Split(tags, ","),
	}

	if tags == "" {
		filter.Tags = nil
	}

	caseList := s.caseMgr.List(filter)
	s.writeJSON(w, http.StatusOK, Response{
		Success:   true,
		Data:      caseList,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleCaseDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/api/v1/cases/")
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "case name required")
		return
	}

	c, err := s.caseMgr.Get(name)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "case not found")
		return
	}

	s.writeJSON(w, http.StatusOK, Response{
		Success:   true,
		Data:      c,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleScenarios(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	scenarioList := s.scenarios.List()
	s.writeJSON(w, http.StatusOK, Response{
		Success:   true,
		Data:      scenarioList,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleScenarioDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/api/v1/scenarios/")
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "scenario name required")
		return
	}

	sc, err := s.scenarios.Get(name)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "scenario not found")
		return
	}

	s.writeJSON(w, http.StatusOK, Response{
		Success:   true,
		Data:      sc,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

type ExecuteRequest struct {
	Type   string            `json:"type"`
	Names  []string          `json:"names"`
	Name   string            `json:"name"`
	Params map[string]string `json:"params"`
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var tasks []*adapter.Task
	var caseNames []string
	var scenarioName string

	switch req.Type {
	case "case":
		for _, name := range req.Names {
			c, err := s.caseMgr.Get(name)
			if err != nil {
				continue
			}
			tasks = append(tasks, &adapter.Task{
				ID:         generateID(),
				ScriptPath: c.Path,
				Language:   adapter.Language(c.Language),
				Params:     c.Params,
				Timeout:    c.Timeout,
				Weight:     c.Weight,
				ScriptName: c.Name,
			})
			caseNames = append(caseNames, name)
		}
	case "scenario":
		sc, err := s.scenarios.Get(req.Name)
		if err != nil {
			s.writeError(w, http.StatusNotFound, "scenario not found")
			return
		}
		scenarioName = req.Name
		for _, caseName := range sc.Cases {
			c, err := s.caseMgr.Get(caseName)
			if err != nil {
				continue
			}
			tasks = append(tasks, &adapter.Task{
				ID:         generateID(),
				ScriptPath: c.Path,
				Language:   adapter.Language(c.Language),
				Params:     c.Params,
				Timeout:    c.Timeout,
				Weight:     c.Weight,
				ScriptName: c.Name,
			})
			caseNames = append(caseNames, caseName)
		}
	default:
		s.writeError(w, http.StatusBadRequest, "invalid type, must be 'case' or 'scenario'")
		return
	}

	if len(tasks) == 0 {
		s.writeError(w, http.StatusBadRequest, "no valid tasks to execute")
		return
	}

	execResults, err := s.engine.ExecuteBatch(r.Context(), tasks)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	matrix, err := s.processor.Process(execResults)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	record := &HistoryRecord{
		ExecutionID: matrix.ExecutionID,
		Timestamp:   matrix.Timestamp.Format(time.RFC3339),
		Status:      getStatusFromSummary(matrix.Summary),
		Trigger:     "web",
		Scenario:    scenarioName,
		Cases:       caseNames,
		Summary: &HistorySummary{
			TotalScripts:    matrix.Summary.TotalScripts,
			SuccessCount:    matrix.Summary.SuccessCount,
			FailureCount:    matrix.Summary.FailureCount,
			WarningCount:    matrix.Summary.WarningCount,
			AverageScore:    matrix.Summary.AverageScore,
			WeightedAverage: matrix.Summary.WeightedAverage,
		},
		Result: matrix,
	}

	s.history.Save(record)

	s.writeJSON(w, http.StatusOK, Response{
		Success:   true,
		Data:      matrix,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		limitStr := r.URL.Query().Get("limit")
		limit := 50
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil {
				limit = l
			}
		}

		records, err := s.history.List(limit)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		s.writeJSON(w, http.StatusOK, Response{
			Success:   true,
			Data:      records,
			Timestamp: time.Now().Format(time.RFC3339),
		})

	case http.MethodDelete:
		if err := s.history.ClearAll(); err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		s.writeJSON(w, http.StatusOK, Response{
			Success:   true,
			Data:      "all history cleared",
			Timestamp: time.Now().Format(time.RFC3339),
		})

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleHistoryDetail(w http.ResponseWriter, r *http.Request) {
	reqPath := strings.TrimPrefix(r.URL.Path, "/api/v1/history/")
	parts := strings.SplitN(reqPath, "/", 2)
	executionID := parts[0]

	if len(parts) > 1 {
		action := parts[1]
		switch action {
		case "export":
			format := r.URL.Query().Get("format")
			if format == "" {
				format = "json"
			}

			data, err := s.history.Export(executionID, format)
			if err != nil {
				s.writeError(w, http.StatusNotFound, "history not found")
				return
			}

			contentType := "application/json"
			if format == "yaml" {
				contentType = "text/yaml"
			}

			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=history_%s.%s", executionID, format))
			w.Write(data)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		record, err := s.history.Get(executionID)
		if err != nil {
			s.writeError(w, http.StatusNotFound, "history not found")
			return
		}

		s.writeJSON(w, http.StatusOK, Response{
			Success:   true,
			Data:      record,
			Timestamp: time.Now().Format(time.RFC3339),
		})

	case http.MethodDelete:
		if err := s.history.Delete(executionID); err != nil {
			s.writeError(w, http.StatusNotFound, "history not found")
			return
		}

		s.writeJSON(w, http.StatusOK, Response{
			Success:   true,
			Data:      "history deleted",
			Timestamp: time.Now().Format(time.RFC3339),
		})

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func generateID() string {
	return fmt.Sprintf("exec-%d", time.Now().UnixNano())
}

func getStatusFromSummary(summary result.MatrixSummary) string {
	if summary.FailureCount > 0 {
		return "failure"
	}
	if summary.WarningCount > 0 {
		return "warning"
	}
	return "success"
}

// handleExecutionDetail 获取执行结果详情
func (s *Server) handleExecutionDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// 提取execution ID
	execID := strings.TrimPrefix(r.URL.Path, "/api/v1/execution/")
	if execID == "" {
		s.writeError(w, http.StatusBadRequest, "execution id required")
		return
	}

	// 获取历史记录
	record, err := s.history.Get(execID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "execution not found")
		return
	}

	s.writeJSON(w, http.StatusOK, Response{
		Success:   true,
		Data:      record,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// handleUserRole 获取当前用户角色
func (s *Server) handleUserRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	role := GetRole(r)
	if role == "" {
		role = "user"
	}

	s.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"role": role,
		},
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// handleSkill 处理Skill相关请求
func (s *Server) handleSkill(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// 判断路由类型
	if strings.Contains(path, "/skill/history/") {
		// 获取指定版本
		if r.Method == http.MethodGet {
			s.skillMgr.HandleSkillHistoryDetail(w, r)
		}
		return
	}

	if strings.HasSuffix(path, "/skill/history") {
		// 获取版本历史
		if r.Method == http.MethodGet {
			s.skillMgr.HandleSkillHistory(w, r)
		}
		return
	}

	if strings.HasSuffix(path, "/skill/rollback") {
		// 回滚
		if r.Method == http.MethodPost {
			s.skillMgr.HandleSkillRollback(w, r)
		}
		return
	}

	if strings.HasSuffix(path, "/skill") {
		// Skill CRUD
		switch r.Method {
		case http.MethodGet:
			s.skillMgr.HandleSkillGet(w, r)
		case http.MethodPut:
			s.skillMgr.HandleSkillUpdate(w, r)
		default:
			s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}
}

// handleSkillHistory 处理版本历史列表
func (s *Server) handleSkillHistory(w http.ResponseWriter, r *http.Request) {
	s.handleSkill(w, r)
}

// handleSkillHistoryDetail 处理版本详情
func (s *Server) handleSkillHistoryDetail(w http.ResponseWriter, r *http.Request) {
	s.handleSkill(w, r)
}

// handleSkillRollback 处理回滚
func (s *Server) handleSkillRollback(w http.ResponseWriter, r *http.Request) {
	s.handleSkill(w, r)
}

// handleKB 处理KB配置请求
func (s *Server) handleKB(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Skill相关的路由处理
	if strings.Contains(path, "/skill/history/") {
		// 获取指定版本
		if r.Method == http.MethodGet {
			s.skillMgr.HandleSkillHistoryDetail(w, r)
			return
		}
	}

	if strings.HasSuffix(path, "/skill/history") {
		// 获取版本历史
		if r.Method == http.MethodGet {
			s.skillMgr.HandleSkillHistory(w, r)
			return
		}
	}

	if strings.HasSuffix(path, "/skill/rollback") {
		// 回滚
		if r.Method == http.MethodPost {
			s.skillMgr.HandleSkillRollback(w, r)
			return
		}
	}

	if strings.HasSuffix(path, "/skill") {
		// Skill CRUD
		switch r.Method {
		case http.MethodGet:
			s.skillMgr.HandleSkillGet(w, r)
		case http.MethodPut:
			s.skillMgr.HandleSkillUpdate(w, r)
		default:
			s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// KB配置请求 (不是skill相关的)
	if !strings.Contains(path, "/skill") && strings.Contains(path, "/kb/") {
		if r.Method == http.MethodGet {
			s.skillMgr.HandleKBConfig(w, r, s.caseMgr)
			return
		}
	}
}