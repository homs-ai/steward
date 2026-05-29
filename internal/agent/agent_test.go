package agent

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/k/steward/internal/config"
	"github.com/k/steward/internal/feature"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input string
		min   int
	}{
		{"", 0},
		{"hello world", 3},
		{"a quick brown fox jumps over the lazy dog", 8},
		{"Hello\nWorld\nThis is a longer text with more content for testing", 10},
	}
	for _, tt := range tests {
		got := estimateTokens(tt.input)
		if got < tt.min {
			t.Errorf("estimateTokens(%q) = %d, want >= %d", tt.input, got, tt.min)
		}
	}
}

func TestEstimateTokensEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"single character", "a"},
		{"whitespace only", "   \n  \t  "},
		{"unicode multi-byte", "你好世界"},
		{"very long", strings.Repeat("hello world ", 10000)},
	}
	for _, tt := range tests {
		got := estimateTokens(tt.input)
		if got < 0 {
			t.Errorf("%s: estimateTokens returned negative %d", tt.name, got)
		}
	}
}

func TestCheckAgent(t *testing.T) {
	CheckAgent("claude")
	CheckAgent("opencode")
	CheckAgent("aider")
	CheckAgent("nonexistent-binary-xyz123")
}

func TestAvailableAgents(t *testing.T) {
	agents := AvailableAgents()
	if agents == nil {
		t.Error("expected non-nil slice")
	}
}

func TestBuildAgentArgs(t *testing.T) {
	promptText := "test prompt text"

	tests := []struct {
		name       string
		agentName  string
		promptFlag string
		expectLen  int
		checkArg   string
	}{
		{"opencode uses --prompt flag", "opencode", "--prompt", 2, "--prompt"},
		{"claude uses -p flag", "claude", "-p", 2, "-p"},
		{"claude-code uses -p flag", "claude-code", "-p", 2, "-p"},
		{"aider uses --message flag", "aider", "--message", 2, "--message"},
		{"opencode falls back when prompt_flag empty", "opencode", "", 2, "--prompt"},
		{"claude falls back when prompt_flag empty", "claude", "", 2, "-p"},
		{"claude-code falls back when prompt_flag empty", "claude-code", "", 2, "-p"},
		{"aider falls back when prompt_flag empty", "aider", "", 2, "--message"},
		{"unknown agent returns empty args", "custom-agent", "", 0, ""},
	}

	for _, tt := range tests {
		agentCfg := &config.AgentConfig{PromptFlag: tt.promptFlag}
		r := &InteractiveRunner{Config: &config.Config{}}
		args := r.buildAgentArgs(agentCfg, tt.agentName, "brainstorm", promptText)
		if len(args) != tt.expectLen {
			t.Errorf("%s: expected %d args, got %d: %v", tt.name, tt.expectLen, len(args), args)
		}
		if tt.checkArg != "" && (len(args) == 0 || args[0] != tt.checkArg) {
			t.Errorf("%s: expected first arg %q, got %v", tt.name, tt.checkArg, args)
		}
		if tt.checkArg != "" && len(args) > 1 && args[1] != promptText {
			t.Errorf("%s: expected second arg to be prompt text, got %q", tt.name, args[1])
		}
	}
}

func TestNewInteractiveRunner(t *testing.T) {
	cfg := &config.Config{StewardHome: "/tmp/test-steward"}
	r := NewInteractiveRunner(cfg)
	if r == nil {
		t.Fatal("expected non-nil InteractiveRunner")
	}
	if r.Config != cfg {
		t.Error("expected Config to match")
	}
}

func TestConfirmExitYes(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	w.Write([]byte("y\n"))
	w.Close()

	result := confirmExit()
	if !result {
		t.Error("expected true for 'y'")
	}
	os.Stdin = oldStdin
}

func TestConfirmExitYesCapitalized(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	w.Write([]byte("Y\n"))
	w.Close()

	result := confirmExit()
	if !result {
		t.Error("expected true for 'Y'")
	}
	os.Stdin = oldStdin
}

func TestConfirmExitYesFull(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	w.Write([]byte("YES\n"))
	w.Close()

	result := confirmExit()
	if !result {
		t.Error("expected true for 'YES'")
	}
	os.Stdin = oldStdin
}

func TestConfirmExitNo(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	w.Write([]byte("n\n"))
	w.Close()

	result := confirmExit()
	if result {
		t.Error("expected false for 'n'")
	}
	os.Stdin = oldStdin
}

func TestConfirmExitNoFull(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	w.Write([]byte("no\n"))
	w.Close()

	result := confirmExit()
	if result {
		t.Error("expected false for 'no'")
	}
	os.Stdin = oldStdin
}

func TestConfirmExitDefault(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	w.Write([]byte("\n"))
	w.Close()

	result := confirmExit()
	if !result {
		t.Error("expected true (default) for empty input")
	}
	os.Stdin = oldStdin
}

func TestCaptureSessionNonNil(t *testing.T) {
	cfg := &config.Config{StewardHome: "/tmp"}
	f, err := feature.Init(cfg, "test-project", "capture-test")
	if err != nil {
		t.Fatal(err)
	}

	session := CaptureSession(f, "brainstorm", "some output", "/tmp")
	if session == nil {
		t.Fatal("expected non-nil SessionCapture")
	}
	if session.Phase != "brainstorm" {
		t.Errorf("expected phase 'brainstorm', got %q", session.Phase)
	}
	if session.RawOutput != "some output" {
		t.Errorf("expected raw output 'some output', got %q", session.RawOutput)
	}
}

func TestCaptureSessionSaveArtifacts(t *testing.T) {
	home := t.TempDir()
	cfg := &config.Config{StewardHome: home}
	f, err := feature.Init(cfg, "test-project", "artifact-test")
	if err != nil {
		t.Fatal(err)
	}

	session := CaptureSession(f, "brainstorm", "brainstorm output content", "/tmp")
	if err := session.SaveArtifacts(); err != nil {
		t.Fatalf("SaveArtifacts() returned error: %v", err)
	}

	sessionFile := f.Dir + "/brainstorm_session.md"
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		t.Errorf("session file %s was not created", sessionFile)
	}

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "brainstorm") {
		t.Error("expected session file to contain phase name")
	}
	if !strings.Contains(content, "Output Size") {
		t.Error("expected session file to contain Output Size")
	}
	if !strings.Contains(content, "Session Summary") {
		t.Error("expected session file to contain Session Summary header")
	}
}

func TestRunInteractiveUnknownPhase(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configDir := home + "/.config/steward"
	os.MkdirAll(configDir, 0755)
	yamlContent := `
steward_home: ` + home + `
default_agent: test-agent
agents:
  test-agent:
    cmd: echo
phases:
  brainstorm:
    agent: test-agent
`
	os.WriteFile(configDir+"/config.yaml", []byte(yamlContent), 0644)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}

	f, err := feature.Init(cfg, "test-project", "unknown-phase-test")
	if err != nil {
		t.Fatal(err)
	}

	r := NewInteractiveRunner(cfg)
	ctx := context.Background()
	_, err = r.RunInteractive(ctx, f, "nonexistent-phase", "prompt", InteractiveOptions{})
	if err == nil {
		t.Error("expected error for unknown phase")
	}
}

func TestRunInteractiveUnknownAgent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configDir := home + "/.config/steward"
	os.MkdirAll(configDir, 0755)
	yamlContent := `
steward_home: ` + home + `
default_agent: claude-code
agents:
  claude-code:
    cmd: claude
phases:
  brainstorm:
    agent: undefined-agent
`
	os.WriteFile(configDir+"/config.yaml", []byte(yamlContent), 0644)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}

	f, err := feature.Init(cfg, "test-project", "unknown-agent-test")
	if err != nil {
		t.Fatal(err)
	}

	r := NewInteractiveRunner(cfg)
	ctx := context.Background()
	_, err = r.RunInteractive(ctx, f, "brainstorm", "prompt", InteractiveOptions{})
	if err == nil {
		t.Error("expected error for unconfigured agent")
	}
}
