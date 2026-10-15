package config

import (
	"fmt"
	"os"
	"path/filepath"

	"ralph/internal/agent"
	ralphErrors "ralph/internal/errors"

	"github.com/spf13/viper"
)

// Config represents the root configuration of Ralph.
type Config struct {
	Agent      agent.Config `mapstructure:"agent" json:"agent"`
	MaxRetries int          `mapstructure:"max_retries" json:"max_retries"`
}

// DefaultConfig returns the default configuration for V0.1.0.
func DefaultConfig() *Config {
	return &Config{
		Agent: agent.Config{
			Type: "codex",
			Codex: agent.CodexConfig{
				Command: "codex",
				Sandbox: "workspace-write",
			},
			Env: make(map[string]string),
		},
		MaxRetries: 3,
	}
}

// Load loads the configuration from a file or the default ralph.json in the current working directory.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("ralph")
		v.SetConfigType("json")
		v.AddConfigPath(".")
	}

	// Set defaults
	v.SetDefault("agent.type", "codex")
	v.SetDefault("agent.codex.command", "codex")
	v.SetDefault("agent.codex.sandbox", "workspace-write")
	v.SetDefault("max_retries", 3)

	if err := v.ReadInConfig(); err != nil {
		if os.IsNotExist(err) || isConfigNotFoundErr(err) {
			return nil, ralphErrors.NewSystemError(fmt.Errorf("configuration file not found: %w", err))
		}
		return nil, ralphErrors.NewSystemError(fmt.Errorf("failed to read configuration file: %w", err))
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, ralphErrors.NewSystemError(fmt.Errorf("failed to parse configuration file: %w", err))
	}

	// Apply fallback defaults if unmarshaled empty
	if cfg.Agent.Type == "" {
		cfg.Agent.Type = "codex"
	}
	if cfg.Agent.Codex.Command == "" {
		cfg.Agent.Codex.Command = "codex"
	}
	if cfg.Agent.Codex.Sandbox == "" {
		cfg.Agent.Codex.Sandbox = "workspace-write"
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}

	return &cfg, nil
}

// LoadFromBytes loads configuration from raw JSON/YAML bytes (useful for testing).
func LoadFromBytes(data []byte) (*Config, error) {
	tmpDir, err := os.MkdirTemp("", "ralph-config-*")
	if err != nil {
		return nil, ralphErrors.NewSystemError(err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "ralph.json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return nil, ralphErrors.NewSystemError(err)
	}

	return Load(filePath)
}

func isConfigNotFoundErr(err error) bool {
	_, ok := err.(viper.ConfigFileNotFoundError)
	return ok
}
