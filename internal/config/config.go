package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type AgentConfig struct {
	Cmd                string            `mapstructure:"cmd"`
	Phases             []string          `mapstructure:"phases"`
	PromptFlag         string            `mapstructure:"prompt_flag"`
	MaxInputTokens     int               `mapstructure:"max_input_tokens"`
	CostPer1KIn        float64           `mapstructure:"cost_per_1k_in"`
	CostPer1KOut       float64           `mapstructure:"cost_per_1k_out"`
	Env                map[string]string `mapstructure:"env"`
	InteractiveBackend string            `mapstructure:"interactive_backend"`
}

type PhaseConfig struct {
	Agent          string `mapstructure:"agent"`
	MaxInputTokens int    `mapstructure:"max_input_tokens"`
	HardBlock      bool   `mapstructure:"hard_block"`
}

type GitConfig struct {
	AutoBranch     string `mapstructure:"auto_branch"`
	BranchTemplate string `mapstructure:"branch_template"`
	DefaultBranch  string `mapstructure:"default_branch"`
	StashOnDirty   bool   `mapstructure:"stash_on_dirty"`
}

type Config struct {
	StewardHome  string                   `mapstructure:"steward_home"`
	DefaultAgent string                   `mapstructure:"default_agent"`
	Agents       map[string]*AgentConfig  `mapstructure:"agents"`
	Phases       map[string]*PhaseConfig  `mapstructure:"phases"`
	Git          *GitConfig               `mapstructure:"git"`
}

func DefaultConfig() *Config {
	return &Config{
		StewardHome:  filepath.Join(os.Getenv("HOME"), ".steward"),
		DefaultAgent: "claude-code",
		Git: &GitConfig{
			AutoBranch:   "prompt",
			StashOnDirty: false,
		},
		Agents: map[string]*AgentConfig{
			"claude-code": {
				Cmd:            "claude",
				Phases:         []string{"brainstorm", "research", "analysis", "implement", "test"},
				PromptFlag:     "-p",
				MaxInputTokens: 32000,
				CostPer1KIn:    0.003,
				CostPer1KOut:   0.015,
			},
			"opencode": {
				Cmd:            "opencode",
				Phases:         []string{"brainstorm", "research", "analysis", "implement", "test"},
				PromptFlag:     "--prompt",
				MaxInputTokens: 32000,
				CostPer1KIn:    0.002,
				CostPer1KOut:   0.010,
			},
			"aider": {
				Cmd:            "aider",
				Phases:         []string{"implement"},
				PromptFlag:     "--message",
				MaxInputTokens: 24000,
				CostPer1KIn:    0.002,
				CostPer1KOut:   0.008,
			},
		},
		Phases: map[string]*PhaseConfig{
			"init":       {Agent: "claude-code", MaxInputTokens: 8000, HardBlock: false},
			"brainstorm": {Agent: "claude-code", MaxInputTokens: 32000, HardBlock: false},
			"research":   {Agent: "claude-code", MaxInputTokens: 32000, HardBlock: false},
			"analysis":   {Agent: "claude-code", MaxInputTokens: 32000, HardBlock: false},
			"implement":  {Agent: "claude-code", MaxInputTokens: 64000, HardBlock: true},
			"test":       {Agent: "claude-code", MaxInputTokens: 48000, HardBlock: false},
		},
	}
}

func Load() (*Config, error) {
	cfg := DefaultConfig()

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	v.AddConfigPath(filepath.Join(os.Getenv("HOME"), ".config", "steward"))
	v.AddConfigPath(filepath.Join(os.Getenv("HOME"), ".steward"))

	v.SetDefault("steward_home", cfg.StewardHome)
	v.SetDefault("default_agent", cfg.DefaultAgent)
	v.SetDefault("agents", cfg.Agents)
	v.SetDefault("phases", cfg.Phases)
	v.SetDefault("git", cfg.Git)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return cfg, nil
		}
		return nil, err
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}

	if cfg.Git != nil {
		switch cfg.Git.AutoBranch {
		case "prompt", "always", "never":
		case "":
			cfg.Git.AutoBranch = "prompt"
		default:
			return nil, fmt.Errorf("git.auto_branch must be one of: prompt, always, never (got %q)", cfg.Git.AutoBranch)
		}
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "steward")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configDir)

	v.Set("steward_home", cfg.StewardHome)
	v.Set("default_agent", cfg.DefaultAgent)
	v.Set("agents", cfg.Agents)
	v.Set("phases", cfg.Phases)

	if cfg.Git != nil {
		v.Set("git.auto_branch", cfg.Git.AutoBranch)
		v.Set("git.branch_template", cfg.Git.BranchTemplate)
		v.Set("git.default_branch", cfg.Git.DefaultBranch)
		v.Set("git.stash_on_dirty", cfg.Git.StashOnDirty)
	}

	return v.WriteConfigAs(filepath.Join(configDir, "config.yaml"))
}
