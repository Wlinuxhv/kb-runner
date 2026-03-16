package api

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
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
}

func NewServer(cfg *config.Config, log *logger.Logger) *Server {
	return &Server{
		cfg:       cfg,
		log:       log,
		engine:    executor.NewEngine(cfg, log),
		caseMgr:   cases.NewManager(),
		scenarios: scenario.NewManager(),
		history:   NewHistoryManager(cfg),
		processor: processor.NewProcessor(cfg, log),
		router:    http.NewServeMux(),
		auth:      NewAuth(cfg.Server.Token),
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