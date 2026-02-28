package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearConnectorEnv unsets all CONNECTOR_ env vars to prevent cross-test leakage.
// t.Setenv only restores the vars it explicitly sets; other CONNECTOR_ vars from
// the host environment or earlier subtests could still leak through.
func clearConnectorEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		if len(kv) > 10 && kv[:10] == "CONNECTOR_" {
			key := kv[:indexOf(kv, '=')]
			t.Setenv(key, "")
			os.Unsetenv(key)
		}
	}
}

func indexOf(s string, c byte) int {
	for i := range len(s) {
		if s[i] == c {
			return i
		}
	}
	return len(s)
}

func TestLoad_Defaults(t *testing.T) {
	clearConnectorEnv(t)
	// Only the required key — everything else should fall back to defaults.
	t.Setenv("CONNECTOR_SATVOS_API_KEY", "sk_test_key_123")

	cfg, err := Load()
	require.NoError(t, err)

	// SATVOS defaults
	assert.Equal(t, "https://api.satvos.com", cfg.SATVOS.BaseURL)
	assert.Equal(t, "sk_test_key_123", cfg.SATVOS.APIKey)

	// Tally defaults
	assert.Equal(t, "localhost", cfg.Tally.Host)
	assert.Equal(t, 0, cfg.Tally.Port)
	assert.Equal(t, "", cfg.Tally.Company)

	// Sync defaults
	assert.Equal(t, 30, cfg.Sync.IntervalSeconds)
	assert.Equal(t, 50, cfg.Sync.BatchSize)
	assert.Equal(t, 3, cfg.Sync.RetryAttempts)

	// UI defaults
	assert.Equal(t, 8321, cfg.UI.Port)
}

func TestLoad_EnvOverride(t *testing.T) {
	clearConnectorEnv(t)
	t.Setenv("CONNECTOR_SATVOS_API_KEY", "sk_override_key")
	t.Setenv("CONNECTOR_SATVOS_BASE_URL", "https://custom.satvos.dev")
	t.Setenv("CONNECTOR_SYNC_INTERVAL_SECONDS", "60")
	t.Setenv("CONNECTOR_TALLY_HOST", "192.168.1.50")
	t.Setenv("CONNECTOR_TALLY_PORT", "9000")
	t.Setenv("CONNECTOR_UI_PORT", "9999")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "https://custom.satvos.dev", cfg.SATVOS.BaseURL)
	assert.Equal(t, "sk_override_key", cfg.SATVOS.APIKey)
	assert.Equal(t, 60, cfg.Sync.IntervalSeconds)
	assert.Equal(t, "192.168.1.50", cfg.Tally.Host)
	assert.Equal(t, 9000, cfg.Tally.Port)
	assert.Equal(t, 9999, cfg.UI.Port)
}

func TestLoad_MissingAPIKey(t *testing.T) {
	clearConnectorEnv(t)
	// Deliberately do NOT set CONNECTOR_SATVOS_API_KEY.

	cfg, err := Load()
	assert.Nil(t, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "satvos.api_key is required")
}

func TestLoad_IntervalClamp(t *testing.T) {
	clearConnectorEnv(t)
	t.Setenv("CONNECTOR_SATVOS_API_KEY", "sk_clamp_test")
	t.Setenv("CONNECTOR_SYNC_INTERVAL_SECONDS", "2")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 5, cfg.Sync.IntervalSeconds, "interval below 5 should be clamped to 5")
}

func TestLoad_BatchSizeClamp(t *testing.T) {
	clearConnectorEnv(t)
	t.Setenv("CONNECTOR_SATVOS_API_KEY", "sk_batch_test")
	t.Setenv("CONNECTOR_SYNC_BATCH_SIZE", "200")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 50, cfg.Sync.BatchSize, "batch size above 100 should be clamped to 50")
}

func TestLoad_BatchSizeClamp_Zero(t *testing.T) {
	clearConnectorEnv(t)
	t.Setenv("CONNECTOR_SATVOS_API_KEY", "sk_batch_zero")
	t.Setenv("CONNECTOR_SYNC_BATCH_SIZE", "0")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 50, cfg.Sync.BatchSize, "batch size of 0 should be clamped to 50")
}

func TestLoad_RetryAttemptsClamp(t *testing.T) {
	clearConnectorEnv(t)
	t.Setenv("CONNECTOR_SATVOS_API_KEY", "sk_retry_test")
	t.Setenv("CONNECTOR_SYNC_RETRY_ATTEMPTS", "0")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 3, cfg.Sync.RetryAttempts, "retry attempts below 1 should be clamped to 3")
}

func TestLoad_ConfigFile(t *testing.T) {
	clearConnectorEnv(t)

	// Create a temp directory with a connector.yaml config file.
	tmpDir := t.TempDir()
	configContent := `
satvos:
  base_url: "https://staging.satvos.com"
  api_key: "sk_from_file"
tally:
  host: "10.0.0.1"
  port: 7000
  company: "My Company Pvt Ltd"
sync:
  interval_seconds: 45
  batch_size: 25
  retry_attempts: 5
ui:
  port: 4000
`
	err := os.WriteFile(filepath.Join(tmpDir, "connector.yaml"), []byte(configContent), 0644)
	require.NoError(t, err)

	// Change to the temp directory so viper finds the config file in ".".
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "https://staging.satvos.com", cfg.SATVOS.BaseURL)
	assert.Equal(t, "sk_from_file", cfg.SATVOS.APIKey)
	assert.Equal(t, "10.0.0.1", cfg.Tally.Host)
	assert.Equal(t, 7000, cfg.Tally.Port)
	assert.Equal(t, "My Company Pvt Ltd", cfg.Tally.Company)
	assert.Equal(t, 45, cfg.Sync.IntervalSeconds)
	assert.Equal(t, 25, cfg.Sync.BatchSize)
	assert.Equal(t, 5, cfg.Sync.RetryAttempts)
	assert.Equal(t, 4000, cfg.UI.Port)
}
