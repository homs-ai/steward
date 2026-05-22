package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.DefaultAgent != "claude-code" {
		t.Errorf("expected default agent 'claude-code', got %q", cfg.DefaultAgent)
	}
	if len(cfg.Agents) != 3 {
		t.Errorf("expected 3 agents, got %d", len(cfg.Agents))
	}
	if _, ok := cfg.Agents["claude-code"]; !ok {
		t.Error("expected claude-code agent config")
	}
}

func TestLoadDefaultConfig(t *testing.T) {
	// Ensure no config file exists in temp home
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.DefaultAgent != "claude-code" {
		t.Errorf("expected default agent 'claude-code', got %q", cfg.DefaultAgent)
	}
}

func TestLoadCustomConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configDir := filepath.Join(home, ".config", "steward")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	yamlContent := `
default_agent: aider
agents:
  aider:
    cmd: aider
    phases: [implement]
    max_input_tokens: 24000
    cost_per_1k_in: 0.002
    cost_per_1k_out: 0.008
phases:
  implement:
    agent: aider
    max_input_tokens: 24000
    hard_block: true
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.DefaultAgent != "aider" {
		t.Errorf("expected default agent 'aider', got %q", cfg.DefaultAgent)
	}
	if cfg.Phases["implement"].Agent != "aider" {
		t.Errorf("expected implement agent 'aider', got %q", cfg.Phases["implement"].Agent)
	}
	if !cfg.Phases["implement"].HardBlock {
		t.Error("expected implement hard_block to be true")
	}
}

func TestSaveAndLoad(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := DefaultConfig()
	cfg.DefaultAgent = "opencode"

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if loaded.DefaultAgent != "opencode" {
		t.Errorf("expected default agent 'opencode', got %q", loaded.DefaultAgent)
	}
}
