package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestDefaultConfigSkipPermsFlags(t *testing.T) {
	cfg := DefaultConfig()

	wantSkip := map[string]string{
		"claude-code": "--dangerously-skip-permissions",
		"opencode":    "--auto",
		"aider":       "--yes-always",
	}
	for name, want := range wantSkip {
		ac, ok := cfg.Agents[name]
		if !ok {
			t.Fatalf("expected agent %q", name)
		}
		if ac.SkipPermsFlag != want {
			t.Errorf("%s: expected skip_perms_flag %q, got %q", name, want, ac.SkipPermsFlag)
		}
	}

	// opencode takes its prompt positionally, not via a flag.
	if got := cfg.Agents["opencode"].PromptFlag; got != "" {
		t.Errorf("opencode: expected empty prompt_flag (positional), got %q", got)
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

func TestDefaultGitConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Git == nil {
		t.Fatal("expected Git config to be non-nil")
	}
	if cfg.Git.AutoBranch != "prompt" {
		t.Errorf("expected AutoBranch 'prompt', got %q", cfg.Git.AutoBranch)
	}
	if cfg.Git.StashOnDirty {
		t.Error("expected StashOnDirty to be false")
	}
	if cfg.Git.BranchTemplate != "" {
		t.Errorf("expected empty BranchTemplate, got %q", cfg.Git.BranchTemplate)
	}
	if cfg.Git.DefaultBranch != "" {
		t.Errorf("expected empty DefaultBranch, got %q", cfg.Git.DefaultBranch)
	}
}

func TestLoadConfigWithoutGitSection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.Git == nil {
		t.Fatal("expected Git config to be non-nil (defaults)")
	}
	if cfg.Git.AutoBranch != "prompt" {
		t.Errorf("expected AutoBranch 'prompt', got %q", cfg.Git.AutoBranch)
	}
}

func TestLoadConfigWithGitSection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configDir := filepath.Join(home, ".config", "steward")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	yamlContent := `
git:
  auto_branch: always
  stash_on_dirty: true
  branch_template: "feat/{{.Name}}"
  default_branch: develop
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.Git.AutoBranch != "always" {
		t.Errorf("expected AutoBranch 'always', got %q", cfg.Git.AutoBranch)
	}
	if !cfg.Git.StashOnDirty {
		t.Error("expected StashOnDirty to be true")
	}
	if cfg.Git.BranchTemplate != "feat/{{.Name}}" {
		t.Errorf("expected BranchTemplate 'feat/{{.Name}}', got %q", cfg.Git.BranchTemplate)
	}
	if cfg.Git.DefaultBranch != "develop" {
		t.Errorf("expected DefaultBranch 'develop', got %q", cfg.Git.DefaultBranch)
	}
}

func TestLoadConfigInvalidAutoBranch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configDir := filepath.Join(home, ".config", "steward")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	yamlContent := `
git:
  auto_branch: invalid_value
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid auto_branch value")
	}
}

func TestSaveAndLoadWithGitConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := DefaultConfig()
	cfg.Git.AutoBranch = "never"
	cfg.Git.StashOnDirty = true
	cfg.Git.BranchTemplate = "feat/{{.Name}}"
	cfg.Git.DefaultBranch = "main"

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	// Verify the saved YAML contains git config
	savedPath := filepath.Join(home, ".config", "steward", "config.yaml")
	savedData, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}
	if !strings.Contains(string(savedData), "auto_branch: never") {
		t.Logf("Saved config content:\n%s", string(savedData))
		t.Fatal("saved config does not contain 'auto_branch: never'")
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if loaded.Git.AutoBranch != "never" {
		t.Errorf("expected AutoBranch 'never', got %q", loaded.Git.AutoBranch)
	}
	if !loaded.Git.StashOnDirty {
		t.Error("expected StashOnDirty to be true")
	}
	if loaded.Git.BranchTemplate != "feat/{{.Name}}" {
		t.Errorf("expected BranchTemplate 'feat/{{.Name}}', got %q", loaded.Git.BranchTemplate)
	}
	if loaded.Git.DefaultBranch != "main" {
		t.Errorf("expected DefaultBranch 'main', got %q", loaded.Git.DefaultBranch)
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
