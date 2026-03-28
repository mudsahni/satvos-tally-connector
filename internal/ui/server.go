package ui

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/mudsahni/satvos-tally-connector/internal/config"
	"github.com/mudsahni/satvos-tally-connector/internal/store"
	engsync "github.com/mudsahni/satvos-tally-connector/internal/sync"
)

//go:embed static
var staticFiles embed.FS

// StartEngineFunc is called after the setup wizard saves a valid API key.
// It receives the API key and should initialize the sync engine, returning it.
// If it returns an error, the setup page will display the error to the user.
type StartEngineFunc func(apiKey string) (*engsync.Engine, error)

// Server serves the local web UI for setup and status monitoring.
type Server struct {
	port        int
	stateDir    string
	cfg         *config.Config
	startEngine StartEngineFunc // called once after setup saves an API key
	store       *store.LocalStore
	server      *http.Server

	mu     sync.RWMutex
	engine *engsync.Engine // nil until configured
	ctx    context.Context // stored so we can start the engine in the handler
}

// NewServer creates a new UI server on the given port.
// engine may be nil if the connector is in setup mode (no API key configured).
// startEngine is called when the user saves config via the setup wizard; it may be nil
// if the engine is already running.
// localStore may be nil in setup mode (created later by startEngine).
func NewServer(port int, engine *engsync.Engine, stateDir string, cfg *config.Config, startEngine StartEngineFunc, localStore *store.LocalStore) *Server {
	return &Server{
		port:        port,
		engine:      engine,
		stateDir:    stateDir,
		cfg:         cfg,
		startEngine: startEngine,
		store:       localStore,
	}
}

// Start begins serving the web UI. It blocks until the context is canceled
// or the server encounters a fatal error.
func (s *Server) Start(ctx context.Context) error {
	s.ctx = ctx

	mux := http.NewServeMux()

	// Static file serving
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("creating static FS: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	// API endpoints
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/sync", s.handleTriggerSync)
	mux.HandleFunc("/api/config", s.handleSaveConfig)
	mux.HandleFunc("/api/validate-key", s.handleValidateKey)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/reconfigure", s.handleReconfigure)
	mux.HandleFunc("/api/reset", s.handleReset)

	s.server = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", s.port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("[ui] starting web UI on http://localhost:%d", s.port)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
	}()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) getEngine() *engsync.Engine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.engine
}

func (s *Server) setEngine(e *engsync.Engine) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.engine = e
}
