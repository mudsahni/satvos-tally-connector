package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// State holds sync cursors, discovered Tally port, company name, and agent ID.
type State struct {
	OutboundCursor string `json:"outbound_cursor"`
	TallyPort      int    `json:"tally_port"`
	TallyCompany   string `json:"tally_company"`
	AgentID        string `json:"agent_id"`
}

// LocalStore provides JSON file-based local state persistence.
type LocalStore struct {
	path  string
	mu    sync.RWMutex
	state State
}

// New creates a LocalStore backed by a state.json file in the given directory.
// The directory is created if it does not exist. If a state file already exists
// it is loaded; otherwise the store starts with a zero-value State.
func New(dir string) (*LocalStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating state dir: %w", err)
	}
	s := &LocalStore{path: filepath.Join(dir, "state.json")}
	_ = s.load() // ignore if file doesn't exist yet
	return s, nil
}

// Get returns a snapshot of the current state.
func (s *LocalStore) Get() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Update applies fn to the state and persists it to disk.
func (s *LocalStore) Update(fn func(*State)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.state)
	return s.save()
}

func (s *LocalStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.state)
}

func (s *LocalStore) save() error {
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
