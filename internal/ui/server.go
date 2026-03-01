package ui

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/mudsahni/satvos-tally-connector/internal/sync"
)

//go:embed static
var staticFiles embed.FS

// Server serves the local web UI for setup and status monitoring.
type Server struct {
	port   int
	engine *sync.Engine
	server *http.Server
}

// NewServer creates a new UI server on the given port, backed by the sync engine.
func NewServer(port int, engine *sync.Engine) *Server {
	return &Server{port: port, engine: engine}
}

// Start begins serving the web UI. It blocks until the context is canceled
// or the server encounters a fatal error.
func (s *Server) Start(ctx context.Context) error {
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
