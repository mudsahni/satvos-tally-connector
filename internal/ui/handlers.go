package ui

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mudsahni/satvos-tally-connector/internal/cloud"
	"github.com/mudsahni/satvos-tally-connector/internal/config"
	"github.com/mudsahni/satvos-tally-connector/internal/logging"
	"github.com/mudsahni/satvos-tally-connector/internal/sync"
)

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	engine := s.getEngine()
	if engine == nil {
		_ = json.NewEncoder(w).Encode(sync.Status{
			TallyConnected: false,
			LastSyncError:  "Setup required: no API key configured",
		})
		return
	}
	status := engine.GetStatus()
	_ = json.NewEncoder(w).Encode(status)
}

func (s *Server) handleTriggerSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	engine := s.getEngine()
	if engine == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "connector not configured yet"})
		return
	}
	go engine.TriggerSync(s.ctx)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "sync triggered"})
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	// Don't allow reconfiguration if engine is already running.
	if s.getEngine() != nil {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "connector is already configured"})
		return
	}

	var req struct {
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "api_key is required"})
		return
	}

	// Persist the API key to disk.
	if err := config.WriteConfigFile(s.stateDir, apiKey); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to save config: " + err.Error()})
		return
	}

	// If a startEngine callback was provided, initialize and start the engine
	// inline so the user doesn't have to restart the connector.
	if s.startEngine != nil {
		engine, err := s.startEngine(apiKey)
		if err != nil {
			log.Printf("[ui] engine start failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "Config saved, but engine failed to start: " + err.Error() + ". Please restart the connector.",
			})
			return
		}

		s.setEngine(engine)

		// Start the sync engine in the background using the server's context.
		go func() {
			if err := engine.Start(s.ctx); err != nil {
				log.Printf("[ui] sync engine stopped: %v", err)
			}
		}()

		log.Println("[ui] sync engine started via setup wizard")
	}

	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Configuration saved. Sync engine started.",
	})
}

func (s *Server) handleValidateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"valid": "false", "error": "invalid request body"})
		return
	}

	key := strings.TrimSpace(req.APIKey)
	if key == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"valid": "false", "error": "api_key is required"})
		return
	}

	if !strings.HasPrefix(key, "sk_") {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"valid": "false", "error": "API key must start with 'sk_'"})
		return
	}

	// Test the key by attempting registration with SATVOS backend
	testClient := cloud.NewClient(s.cfg.SATVOS.BaseURL, key)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, err := testClient.Register(ctx, cloud.RegisterRequest{
		Version: "validation-check",
		OSInfo:  "validation-check",
	})
	if err != nil {
		errMsg := err.Error()
		// Make error messages user-friendly
		if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "403") || strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "forbidden") {
			errMsg = "API key is invalid or expired"
		} else if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "no such host") || strings.Contains(errMsg, "timeout") {
			errMsg = "Cannot reach SATVOS servers. Check your internet connection."
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"valid": false, "error": errMsg})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{"valid": true})
}

func (s *Server) handleReconfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		NewAPIKey string `json:"new_api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	newKey := strings.TrimSpace(req.NewAPIKey)
	if newKey == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "new_api_key is required"})
		return
	}

	if !strings.HasPrefix(newKey, "sk_") {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "API key must start with 'sk_'"})
		return
	}

	// Validate the key by attempting registration with SATVOS backend
	testClient := cloud.NewClient(s.cfg.SATVOS.BaseURL, newKey)
	vctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, err := testClient.Register(vctx, cloud.RegisterRequest{
		Version: "validation-check",
		OSInfo:  "validation-check",
	})
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "API key validation failed: " + err.Error()})
		return
	}

	// Stop current engine if running
	if engine := s.getEngine(); engine != nil {
		engine.Stop()
	}

	// Write new config
	if err := config.WriteConfigFile(s.stateDir, newKey); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to save config: " + err.Error()})
		return
	}

	// Start new engine
	if s.startEngine != nil {
		engine, err := s.startEngine(newKey)
		if err != nil {
			log.Printf("[ui] engine start failed after reconfigure: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "Config saved, but engine failed to start: " + err.Error() + ". Please restart the connector.",
			})
			return
		}

		s.setEngine(engine)

		go func() {
			if err := engine.Start(s.ctx); err != nil {
				log.Printf("[ui] sync engine stopped after reconfigure: %v", err)
			}
		}()

		log.Println("[ui] sync engine restarted with new API key")
	}

	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "API key updated and sync engine restarted.",
	})
}

func (s *Server) handlePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	engine := s.getEngine()
	if engine == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "connector not configured yet"})
		return
	}
	engine.Pause()
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Sync paused"})
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	engine := s.getEngine()
	if engine == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "connector not configured yet"})
		return
	}
	engine.Resume()
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Sync resumed"})
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	// Stop engine if running
	if engine := s.getEngine(); engine != nil {
		engine.Stop()
	}

	// Delete config file
	if err := config.DeleteConfigFile(s.stateDir); err != nil && !os.IsNotExist(err) {
		log.Printf("[ui] warning: failed to delete config file: %v", err)
	}

	// Reset local state
	if s.store != nil {
		if err := s.store.Reset(); err != nil && !os.IsNotExist(err) {
			log.Printf("[ui] warning: failed to reset local state: %v", err)
		}
	}

	// Clear engine reference
	s.setEngine(nil)

	log.Println("[ui] connection reset by user")

	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Connection reset. Redirecting to setup...",
	})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	lines, err := logging.ReadLastLines(s.stateDir, 200)
	if err != nil {
		http.Error(w, "failed to read logs: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(lines))
}
