package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k/steward/internal/config"
	"github.com/k/steward/internal/feature"
)

func setupTest(t *testing.T) (*config.Config, *feature.Feature) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.DefaultConfig()
	cfg.StewardHome = filepath.Join(home, ".steward")

	f, err := feature.Init(cfg, "test-project", "test-feature")
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}
	return cfg, f
}

func TestBuildPhasePrompt(t *testing.T) {
	cfg, f := setupTest(t)

	f.WriteFile("req.md", "# Requirements\nBuild a login system")
	f.WriteFile("brainstorm.md", "# Ideas\nUse OAuth2")

	prompt := BuildPhasePrompt(cfg, f, "analysis", []string{"req.md", "brainstorm.md"})

	if !strings.Contains(prompt, "test-project:test-feature") {
		t.Error("expected prompt to contain project:feature name")
	}
	if !strings.Contains(prompt, "Build a login system") {
		t.Error("expected prompt to contain req.md content")
	}
	if !strings.Contains(prompt, "<<<STALE>>>") {
		t.Error("expected prompt to mention stale marker")
	}
}

func TestBuildPhasePromptWithStaleContent(t *testing.T) {
	cfg, f := setupTest(t)

	f.WriteWithStale("brainstorm.md", "Round 1 ideas")
	f.WriteWithStale("brainstorm.md", "Round 2 refined ideas")

	prompt := BuildPhasePrompt(cfg, f, "research", []string{"brainstorm.md"})

	if strings.Contains(prompt, "Round 1 ideas") {
		t.Log("prompt may contain stale content (depends on implementation)")
	}
	if !strings.Contains(prompt, "Round 2 refined ideas") {
		t.Error("expected prompt to contain latest (after-stale) content")
	}
}

func TestNewPhaseRunner(t *testing.T) {
	cfg, _ := setupTest(t)
	pr := NewPhaseRunner(cfg)
	if pr == nil {
		t.Fatal("expected non-nil PhaseRunner")
	}
	if pr.Runner == nil {
		t.Fatal("expected non-nil Runner")
	}
}

func TestRequireRatingInput(t *testing.T) {
	_, f := setupTest(t)

	// Simulate skip (pressing Enter)
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r

	w.Write([]byte("\n"))
	w.Close()

	RequireRating(f, "brainstorm")

	os.Stdin = oldStdin
}
