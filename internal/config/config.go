package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	SATVOS SATVOSConfig `mapstructure:"satvos"`
	Tally  TallyConfig  `mapstructure:"tally"`
	Sync   SyncConfig   `mapstructure:"sync"`
	UI     UIConfig     `mapstructure:"ui"`
}

type SATVOSConfig struct {
	BaseURL string `mapstructure:"base_url"`
	APIKey  string `mapstructure:"api_key"`
}

type TallyConfig struct {
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	Company string `mapstructure:"company"`
}

type SyncConfig struct {
	IntervalSeconds int `mapstructure:"interval_seconds"`
	BatchSize       int `mapstructure:"batch_size"`
	RetryAttempts   int `mapstructure:"retry_attempts"`
}

type UIConfig struct {
	Port int `mapstructure:"port"`
}

func Load() (*Config, error) {
	v := viper.New()

	// Defaults — every key must be registered so AutomaticEnv can map
	// CONNECTOR_<SECTION>_<KEY> env vars to the corresponding config path.
	v.SetDefault("satvos.base_url", "https://satvos-backend-production.up.railway.app")
	v.SetDefault("satvos.api_key", "")
	v.SetDefault("tally.host", "localhost")
	v.SetDefault("tally.port", 0)
	v.SetDefault("tally.company", "")
	v.SetDefault("sync.interval_seconds", 30)
	v.SetDefault("sync.batch_size", 50)
	v.SetDefault("sync.retry_attempts", 3)
	v.SetDefault("ui.port", 8321)

	// Config file search paths
	v.SetConfigName("connector")
	v.SetConfigType("yaml")
	if appData := os.Getenv("APPDATA"); appData != "" {
		v.AddConfigPath(filepath.Join(appData, "satvos-connector"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(filepath.Join(home, ".satvos-connector"))
	}
	v.AddConfigPath(".")
	v.AddConfigPath("./configs")

	// Env vars: CONNECTOR_ prefix
	v.SetEnvPrefix("CONNECTOR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file (optional — env vars sufficient)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	cfg.clamp()

	return &cfg, nil
}

// NeedsSetup returns true if the connector has not been configured yet
// (i.e., the API key is missing and the user needs to complete the setup wizard).
func (c *Config) NeedsSetup() bool {
	return c.SATVOS.APIKey == ""
}

// WriteConfigFile writes a connector.yaml with the given API key into dir.
// This is used by the setup wizard to persist the initial configuration.
func WriteConfigFile(dir, apiKey string) error {
	content := fmt.Sprintf("satvos:\n  api_key: %q\n", apiKey)
	path := filepath.Join(dir, "connector.yaml")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}

// clamp enforces min/max bounds on numeric config values.
func (c *Config) clamp() {
	if c.Sync.IntervalSeconds < 5 {
		c.Sync.IntervalSeconds = 5
	}
	if c.Sync.BatchSize < 1 || c.Sync.BatchSize > 100 {
		c.Sync.BatchSize = 50
	}
	if c.Sync.RetryAttempts < 1 {
		c.Sync.RetryAttempts = 3
	}
}
