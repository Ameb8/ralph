package agent

import (
	"fmt"
)

// CodexConfig specifies options for invoking the OpenAI Codex CLI.
type CodexConfig struct {
	Command         string   `mapstructure:"command" json:"command"`
	Model           string   `mapstructure:"model" json:"model"`
	ReasoningEffort string   `mapstructure:"reasoning_effort" json:"reasoning_effort"`
	Sandbox         string   `mapstructure:"sandbox" json:"sandbox"`
	ExtraArgs       []string `mapstructure:"extra_args" json:"extra_args"`
}

// Config specifies the overall agent configuration block in ralph.json.
type Config struct {
	Type  string            `mapstructure:"type" json:"type"`   // e.g., "codex" (default in V0.1.0)
	Codex CodexConfig       `mapstructure:"codex" json:"codex"` // Codex-specific configuration
	Env   map[string]string `mapstructure:"env" json:"env"`     // Environment variables
}

// NewRunner creates an AgentRunner corresponding to the configured agent type.
func NewRunner(cfg *Config) (AgentRunner, error) {
	if cfg == nil {
		return nil, fmt.Errorf("agent config cannot be nil")
	}

	switch cfg.Type {
	case "codex", "": // default to codex in V0.1.0
		return NewCodexRunner(&cfg.Codex, cfg.Env), nil
	default:
		return nil, fmt.Errorf("unsupported agent type %q: V0.1.0 only supports 'codex'", cfg.Type)
	}
}
