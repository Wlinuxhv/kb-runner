package api

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"kb-runnerx/internal/adapter"
	"kb-runnerx/internal/cases"
	"kb-runnerx/internal/executor"
	"kb-runnerx/internal/processor"
	"kb-runnerx/internal/scenario"
	"kb-runnerx/pkg/config"
	"kb-runnerx/pkg/logger"
)

//go:embed web/dist/*
var webFS embed.FS

type Server struct {
	cfg       *config.Config
	log       *logger.Logger
	engine    *executor.Engine
	caseMgr   *cases.Manager
	scenarios *scenario.Manager
	history   *HistoryManager
	processor *processor.Processor
	router    *http.ServeMux
	httpSrv   *http.Server
	auth      *Auth
	skillMgr  *SkillManager
}

func NewServer(cfg *config.Config, log *logger.Logger) *Server {
	// 获取工作目录
	workDir := cfg.Execution.WorkDir
	if workDir == "" {
		workDir = "./workspace"
	}

	caseMgr := cases.NewManager()
	scenariosMgr := scenario.NewManager()

	// 加载cases和scenarios
	casesDir := filepath.Join(workDir, "cases")
	if _, err := os.Stat(casesDir); err == nil {
		if err := caseMgr.LoadFromDirectory(casesDir); err != nil {
			log.Error("Failed to load cases", "error", err)
		}
	}

	scenariosDir := filepath.Join(workDir, "scenarios")
	if _, err := os.Stat(scenariosDir); err == nil {
		if err := scenariosMgr.LoadFromDirectory(scenariosDir); err != nil {
			log.Error("Failed to load scenarios", "error", err)
		}
	}

	return &Server{
		cfg:       cfg,
		log:       log,
		engine:    executor.NewEngine(cfg, log),
		caseMgr:   caseMgr,
		scenarios: scenariosMgr,
		history:   NewHistoryManager(cfg),
		processor: processor.NewProcessor(cfg, log),
		router:    http.NewServeMux(),
		auth:      NewAuth(cfg.Server.Token),
		skillMgr:  NewSkillManager(workDir),
	}
}

func (s *Server) Engine() *executor.Engine {
	return s.engine
}

func (s *Server) CaseManager() *cases.Manager {
	return s.caseMgr
}

func (s *Server) ScenarioManager() *scenario.Manager {
	return s.scenarios
}

func (s *Server) RegisterAdapter(language adapter.Language, a adapter.Adapter) {
	s.engine.RegisterAdapter(language, a)
}

func (s *Server) Start(addr string) error {
	s.setupRoutes()

	s.httpSrv = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		WriteTimeout: s.cfg.Server.WriteTimeout,
	}

	s.log.Info("Starting web server", "address", addr)

	return s.httpSrv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpSrv != nil {
		return s.httpSrv.Shutdown(ctx)
	}
	return nil
}

func (s *Server) setupRoutes() {
	s.router.HandleFunc("/api/v1/cases", s.handleCases)
	s.router.HandleFunc("/api/v1/cases/", s.handleCaseDetail)
	s.router.HandleFunc("/api/v1/scenarios", s.handleScenarios)
	s.router.HandleFunc("/api/v1/scenarios/", s.handleScenarioDetail)
	s.router.HandleFunc("/api/v1/execute", s.handleExecute)
	s.router.HandleFunc("/api/v1/history", s.handleHistory)
	s.router.HandleFunc("/api/v1/history/", s.handleHistoryDetail)
	s.router.HandleFunc("/api/v1/health", s.handleHealth)

	// 执行结果API
	s.router.HandleFunc("/api/v1/execution/", s.handleExecutionDetail)

	// Skill/KB管理API - 使用统一的handler
	s.router.HandleFunc("/api/v1/kb/", s.handleKB)

	// 用户角色API
	s.router.HandleFunc("/api/v1/user/role", s.handleUserRole)

	if s.auth != nil {
		s.router.HandleFunc("/login", s.auth.HandleLogin)
	}

	distFS, _ := fs.Sub(webFS, "web/dist")
	staticHandler := http.FileServer(http.FS(distFS))

	if s.auth != nil {
		s.router.Handle("/", s.auth.Middleware(staticHandler))
	} else {
		s.router.Handle("/", staticHandler)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}