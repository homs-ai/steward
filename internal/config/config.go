package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// NOTE: every field carries matching `mapstructure` AND `yaml` tags. viper
// decodes config files via mapstructure but Save() serializes structs to YAML
// via yaml.v3; without the yaml tags the two use different key names
// (e.g. "prompt_flag" vs "promptflag") and every multi-word field silently
// round-trips to its zero value.
type AgentConfig struct {
	Cmd                string            `mapstructure:"cmd" yaml:"cmd"`
	Phases             []string          `mapstructure:"phases" yaml:"phases"`
	PromptFlag         string            `mapstructure:"prompt_flag" yaml:"prompt_flag"`
	SkipPermsFlag      string            `mapstructure:"skip_perms_flag" yaml:"skip_perms_flag"`
	MaxInputTokens     int               `mapstructure:"max_input_tokens" yaml:"max_input_tokens"`
	CostPer1KIn        float64           `mapstructure:"cost_per_1k_in" yaml:"cost_per_1k_in"`
	CostPer1KOut       float64           `mapstructure:"cost_per_1k_out" yaml:"cost_per_1k_out"`
	Env                map[string]string `mapstructure:"env" yaml:"env"`
	InteractiveBackend string            `mapstructure:"interactive_backend" yaml:"interactive_backend"`
}

type PhaseConfig struct {
	Agent          string `mapstructure:"agent" yaml:"agent"`
	MaxInputTokens int    `mapstructure:"max_input_tokens" yaml:"max_input_tokens"`
	HardBlock      bool   `mapstructure:"hard_block" yaml:"hard_block"`
}

type GitConfig struct {
	AutoBranch     string `mapstructure:"auto_branch" yaml:"auto_branch"`
	BranchTemplate string `mapstructure:"branch_template" yaml:"branch_template"`
	DefaultBranch  string `mapstructure:"default_branch" yaml:"default_branch"`
	StashOnDirty   bool   `mapstructure:"stash_on_dirty" yaml:"stash_on_dirty"`
}

type RAGConfig struct {
	Backend   string `mapstructure:"backend" yaml:"backend"`
	ModelPath string `mapstructure:"model_path" yaml:"model_path"`
	StorePath string `mapstructure:"store_path" yaml:"store_path"`
	Enabled   bool   `mapstructure:"enabled" yaml:"enabled"`
}

type Config struct {
	StewardHome  string                  `mapstructure:"steward_home" yaml:"steward_home"`
	DefaultAgent string                  `mapstructure:"default_agent" yaml:"default_agent"`
	Agents       map[string]*AgentConfig `mapstructure:"agents" yaml:"agents"`
	Phases       map[string]*PhaseConfig `mapstructure:"phases" yaml:"phases"`
	Git          *GitConfig              `mapstructure:"git" yaml:"git"`
	RAG          *RAGConfig              `mapstructure:"rag" yaml:"rag"`
}

func DefaultConfig() *Config {
	return &Config{
		StewardHome:  filepath.Join(os.Getenv("HOME"), ".steward"),
		DefaultAgent: "claude-code",
		Git: &GitConfig{
			AutoBranch:   "prompt",
			StashOnDirty: false,
		},
		RAG: &RAGConfig{
			Backend:   "goformer",
			ModelPath: filepath.Join(os.Getenv("HOME"), ".steward", "models", "goformer"),
			StorePath: filepath.Join(os.Getenv("HOME"), ".steward", "rag", "store"),
			Enabled:   true,
		},
		Agents: map[string]*AgentConfig{
			"claude-code": {
				Cmd:            "claude",
				Phases:         []string{"brainstorm", "research", "analysis", "implement", "test"},
				PromptFlag:     "-p",
				SkipPermsFlag:  "--dangerously-skip-permissions",
				MaxInputTokens: 32000,
				CostPer1KIn:    0.003,
				CostPer1KOut:   0.015,
			},
			"opencode": {
				Cmd:            "opencode",
				Phases:         []string{"brainstorm", "research", "analysis", "implement", "test"},
				PromptFlag:     "",
				SkipPermsFlag:  "--auto",
				MaxInputTokens: 32000,
				CostPer1KIn:    0.002,
				CostPer1KOut:   0.010,
			},
			"aider": {
				Cmd:            "aider",
				Phases:         []string{"implement"},
				PromptFlag:     "--message",
				SkipPermsFlag:  "--yes-always",
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

	// viper replaces (does not deep-merge) nested maps from a config file over
	// the defaults set via SetDefault, so a field added after the file was
	// written — e.g. skip_perms_flag — comes back empty. Backfill known agents'
	// permission flags from the built-in defaults when the file omits them.
	defaults := DefaultConfig()
	for name, ac := range cfg.Agents {
		if ac == nil {
			continue
		}
		if ac.SkipPermsFlag == "" {
			if def, ok := defaults.Agents[name]; ok {
				ac.SkipPermsFlag = def.SkipPermsFlag
			}
		}
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

	if cfg.RAG != nil {
		v.Set("rag.backend", cfg.RAG.Backend)
		v.Set("rag.model_path", cfg.RAG.ModelPath)
		v.Set("rag.store_path", cfg.RAG.StorePath)
		v.Set("rag.enabled", cfg.RAG.Enabled)
	}

	return v.WriteConfigAs(filepath.Join(configDir, "config.yaml"))
}
