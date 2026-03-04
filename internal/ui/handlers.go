package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mudsahni/satvos-tally-connector/internal/config"
	"github.com/mudsahni/satvos-tally-connector/internal/sync"
)

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if s.engine == nil {
		_ = json.NewEncoder(w).Encode(sync.Status{
			TallyConnected: false,
			LastSyncError:  "Setup required: no API key configured",
		})
		return
	}
	status := s.engine.GetStatus()
	_ = json.NewEncoder(w).Encode(status)
}

func (s *Server) handleTriggerSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if s.engine == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "connector not configured yet"})
		return
	}
	go s.engine.TriggerSync(context.Background())
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "sync triggered"})
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
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
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "api_key is required"})
		return
	}

	if err := config.WriteConfigFile(s.stateDir, apiKey); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to save config: " + err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "saved",
		"message": "Configuration saved. Please restart the connector.",
	})
}
