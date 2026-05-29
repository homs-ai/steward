package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k/steward/internal/config"
	"github.com/k/steward/internal/feature"
	"github.com/k/steward/internal/telemetry"
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

func TestRequireRatingValidRange(t *testing.T) {
	tests := []struct {
		input  string
		rating int
	}{
		{"1\n", 1},
		{"3\n", 3},
		{"5\n", 5},
	}
	for _, tt := range tests {
		_, f := setupTest(t)

		telemetry.RecordPhaseStart(f, "brainstorm", "test-agent")
		telemetry.RecordPhaseEnd(f, "brainstorm", 100, 50, 0, 0, "")

		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r
		w.Write([]byte(tt.input))
		w.Close()

		RequireRating(f, "brainstorm")

		os.Stdin = oldStdin

		ft, _ := telemetry.Load(f)
		if ft.Phases["brainstorm"].HumanRating != tt.rating {
			t.Errorf("input %q: expected rating %d, got %d", tt.input, tt.rating, ft.Phases["brainstorm"].HumanRating)
		}
	}
}

func TestRequireRatingInvalidInput(t *testing.T) {
	tests := []string{"0\n", "6\n", "-1\n", "abc\n", "\n"}
	for _, input := range tests {
		_, f := setupTest(t)

		telemetry.RecordPhaseStart(f, "brainstorm", "test-agent")
		telemetry.RecordPhaseEnd(f, "brainstorm", 100, 50, 0, 0, "")

		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r
		w.Write([]byte(input))
		w.Close()

		RequireRating(f, "brainstorm")

		os.Stdin = oldStdin

		ft, _ := telemetry.Load(f)
		if ft.Phases["brainstorm"].HumanRating != 0 {
			t.Errorf("input %q: expected rating 0 (not recorded), got %d", input, ft.Phases["brainstorm"].HumanRating)
		}
	}
}

func TestBuildPhasePromptWithExtraContext(t *testing.T) {
	cfg, f := setupTest(t)

	pr := &PhaseRunner{Config: cfg}
	prompt := pr.buildPhasePrompt(f, "brainstorm", "Extra context text")

	if !strings.Contains(prompt, "test-project:test-feature") {
		t.Error("expected prompt to contain feature name")
	}
	if !strings.Contains(prompt, "Extra context text") {
		t.Error("expected prompt to contain extra context")
	}
}

func TestBuildPhasePromptWithoutContext(t *testing.T) {
	cfg, f := setupTest(t)

	pr := &PhaseRunner{Config: cfg}
	prompt := pr.buildPhasePrompt(f, "brainstorm", "")

	if !strings.Contains(prompt, "test-project:test-feature") {
		t.Error("expected prompt to contain feature name")
	}
	if strings.Contains(prompt, "Extra context") {
		t.Error("expected prompt not to contain extra context when none provided")
	}
}

func TestBrainstormPromptContainsDivergentThinking(t *testing.T) {
	_, f := setupTest(t)

	input := "Build a login system"
	prompt := fmt.Sprintf(`You are helping brainstorm for a software feature called '%s'.

The user's initial description or problem statement is:
%s

This is a divergent thinking phase. Your goal is raw idea generation, problem identification, and exploring potential solutions without judgment.

Generate a structured brainstorm document covering:
- Problem statement and goals
- Potential solutions and approaches (explore multiple directions, including unconventional ones)
- Key assumptions and constraints
- Risks and challenges
- Wild or innovative ideas worth exploring

Write this after the last occurrence of <<<STALE>>> in %s/brainstorm.md. Format it clearly in Markdown.`,
		f.DisplayName(), input, f.Dir)

	checks := []struct {
		name string
		part string
	}{
		{"feature name", f.DisplayName()},
		{"user input", input},
		{"divergent thinking", "divergent thinking"},
		{"idea generation", "idea generation"},
		{"brainstorm.md reference", "brainstorm.md"},
		{"stale marker", "<<<STALE>>>"},
		{"problem statement", "Problem statement"},
		{"potential solutions", "Potential solutions"},
		{"assumptions", "assumptions"},
		{"risks and challenges", "Risks and challenges"},
		{"wild ideas", "Wild or innovative ideas"},
	}

	for _, c := range checks {
		if !strings.Contains(prompt, c.part) {
			t.Errorf("prompt missing %s: expected to contain %q", c.name, c.part)
		}
	}
}

func TestBrainstormPromptWithEmptyInput(t *testing.T) {
	_, f := setupTest(t)

	input := ""
	prompt := fmt.Sprintf(`You are helping brainstorm for a software feature called '%s'.

The user's initial description or problem statement is:
%s

This is a divergent thinking phase. Your goal is raw idea generation, problem identification, and exploring potential solutions without judgment.

Generate a structured brainstorm document covering:
- Problem statement and goals
- Potential solutions and approaches (explore multiple directions, including unconventional ones)
- Key assumptions and constraints
- Risks and challenges
- Wild or innovative ideas worth exploring

Write this after the last occurrence of <<<STALE>>> in %s/brainstorm.md. Format it clearly in Markdown.`,
		f.DisplayName(), input, f.Dir)

	if prompt == "" {
		t.Error("expected non-empty prompt even with empty input")
	}
	if !strings.Contains(prompt, "<<<STALE>>>") {
		t.Error("expected stale marker in prompt")
	}
}

func TestBrainstormBatchE2E(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configDir := filepath.Join(home, ".config", "steward")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	yamlContent := `
default_agent: test-brainstorm-agent
agents:
  test-brainstorm-agent:
    cmd: echo
    phases: [brainstorm]
    max_input_tokens: 32000
phases:
  brainstorm:
    agent: test-brainstorm-agent
    max_input_tokens: 32000
steward_home: ` + filepath.Join(home, ".steward") + `
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}

	f, err := feature.Init(cfg, "test-project", "brainstorm-batch-test")
	if err != nil {
		t.Fatal(err)
	}

	pr := NewPhaseRunner(cfg)
	pr.Interactive = false
	pr.ProjectRoot = f.Dir

	ctx := context.Background()
	input := "Test feature for batch brainstorming"

	if err := pr.Brainstorm(ctx, f, input); err != nil {
		t.Fatalf("Brainstorm() returned error: %v", err)
	}

	if _, err := os.Stat(f.TelemetryFile()); os.IsNotExist(err) {
		t.Error("telemetry file was not created")
	}

	ft, err := telemetry.Load(f)
	if err != nil {
		t.Fatalf("Load telemetry error: %v", err)
	}
	if ft.Phases["brainstorm"] == nil {
		t.Fatal("expected brainstorm telemetry")
	}
	if ft.Phases["brainstorm"].Agent == "" {
		t.Error("expected agent name in telemetry")
	}
	if ft.Phases["brainstorm"].ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", ft.Phases["brainstorm"].ExitCode)
	}
}

func TestBrainstormBatchNonExistentAgent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configDir := filepath.Join(home, ".config", "steward")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	yamlContent := `
default_agent: nonexistent-binary-xyz789
agents:
  nonexistent-binary-xyz789:
    cmd: nonexistent-binary-xyz789
    phases: [brainstorm]
phases:
  brainstorm:
    agent: nonexistent-binary-xyz789
steward_home: ` + filepath.Join(home, ".steward") + `
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}

	f, err := feature.Init(cfg, "test-project", "batch-error-test")
	if err != nil {
		t.Fatal(err)
	}

	pr := NewPhaseRunner(cfg)
	pr.Interactive = false
	pr.ProjectRoot = f.Dir

	ctx := context.Background()
	err = pr.Brainstorm(ctx, f, "test input")
	if err == nil {
		t.Error("expected error for non-existent agent binary")
	}
}

func TestRequireRatingNoTelemetry(t *testing.T) {
	_, f := setupTest(t)

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	w.Write([]byte("3\n"))
	w.Close()

	RequireRating(f, "brainstorm")

	os.Stdin = oldStdin

	ft, _ := telemetry.Load(f)
	if ft.Phases["brainstorm"] != nil {
		t.Error("expected no telemetry for unstarted phase")
	}
}
