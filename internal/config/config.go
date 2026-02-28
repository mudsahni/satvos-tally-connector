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
	v.SetDefault("satvos.base_url", "https://api.satvos.com")
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

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.SATVOS.APIKey == "" {
		return fmt.Errorf("satvos.api_key is required (set CONNECTOR_SATVOS_API_KEY or in config file)")
	}
	if c.Sync.IntervalSeconds < 5 {
		c.Sync.IntervalSeconds = 5
	}
	if c.Sync.BatchSize < 1 || c.Sync.BatchSize > 100 {
		c.Sync.BatchSize = 50
	}
	if c.Sync.RetryAttempts < 1 {
		c.Sync.RetryAttempts = 3
	}
	return nil
}
