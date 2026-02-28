package store

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStore_NewCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "state")

	_, err := New(dir)
	require.NoError(t, err)

	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestLocalStore_SaveLoad(t *testing.T) {
	dir := t.TempDir()

	// Create store and update state.
	s1, err := New(dir)
	require.NoError(t, err)

	err = s1.Update(func(st *State) {
		st.OutboundCursor = "cursor-42"
		st.TallyPort = 9000
		st.TallyCompany = "ACME Corp"
		st.AgentID = "agent-abc"
	})
	require.NoError(t, err)

	// Create a new store from the same directory — it should load persisted state.
	s2, err := New(dir)
	require.NoError(t, err)

	got := s2.Get()
	assert.Equal(t, "cursor-42", got.OutboundCursor)
	assert.Equal(t, 9000, got.TallyPort)
	assert.Equal(t, "ACME Corp", got.TallyCompany)
	assert.Equal(t, "agent-abc", got.AgentID)
}

func TestLocalStore_DefaultState(t *testing.T) {
	dir := t.TempDir()

	s, err := New(dir)
	require.NoError(t, err)

	got := s.Get()
	assert.Equal(t, State{}, got)
}

func TestLocalStore_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()

	s, err := New(dir)
	require.NoError(t, err)

	var wg sync.WaitGroup
	const n = 50

	// Concurrent writers.
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Update(func(st *State) {
				st.TallyPort = 9000
			})
		}()
	}

	// Concurrent readers.
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Get()
		}()
	}

	wg.Wait()

	// After all goroutines finish, the state should still be consistent.
	got := s.Get()
	assert.Equal(t, 9000, got.TallyPort)
}
