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

	// These cases run in manual mode so no permission flag is prepended and the
	// prompt-flag placement can be asserted directly (first arg = flag or prompt).
	//
	// Interactive claude/opencode take the prompt positionally regardless of the
	// configured PromptFlag: that flag is the *batch* (non-interactive) flag
	// (e.g. claude's -p/--print), which would run a one-shot and kill the PTY
	// session. Only aider uses a flag interactively.
	tests := []struct {
		name       string
		agentName  string
		promptFlag string
		expectLen  int
		checkArg   string
	}{
		{"opencode ignores configured -p-style flag, stays positional", "opencode", "--prompt", 1, ""},
		{"claude ignores -p flag, stays positional", "claude", "-p", 1, ""},
		{"claude-code ignores -p flag, stays positional", "claude-code", "-p", 1, ""},
		{"aider uses --message flag", "aider", "--message", 2, "--message"},
		{"opencode positional when prompt_flag empty", "opencode", "", 1, ""},
		{"claude positional when prompt_flag empty", "claude", "", 1, ""},
		{"claude-code positional when prompt_flag empty", "claude-code", "", 1, ""},
		{"aider falls back when prompt_flag empty", "aider", "", 2, "--message"},
		{"unknown agent returns positional prompt only", "custom-agent", "", 1, ""},
	}

	for _, tt := range tests {
		agentCfg := &config.AgentConfig{PromptFlag: tt.promptFlag}
		r := &InteractiveRunner{Config: &config.Config{}, Manual: true}
		args := r.buildAgentArgs(agentCfg, tt.agentName, "brainstorm", promptText)
		if len(args) != tt.expectLen {
			t.Errorf("%s: expected %d args, got %d: %v", tt.name, tt.expectLen, len(args), args)
		}
		if tt.checkArg != "" && (len(args) == 0 || args[0] != tt.checkArg) {
			t.Errorf("%s: expected first arg %q, got %v", tt.name, tt.checkArg, args)
		}
		// Prompt text is always the final positional argument.
		if len(args) > 0 && args[len(args)-1] != promptText {
			t.Errorf("%s: expected last arg to be prompt text, got %q", tt.name, args[len(args)-1])
		}
	}
}

func TestBuildAgentArgsAutoPrependsSkipFlag(t *testing.T) {
	t.Setenv(forceManualEnv, "")
	promptText := "hello"

	// opencode: --auto is prepended, prompt stays positional.
	oc := &config.AgentConfig{SkipPermsFlag: "--auto"}
	r := &InteractiveRunner{Config: &config.Config{}, Manual: false}
	args := r.buildAgentArgs(oc, "opencode", "brainstorm", promptText)
	want := []string{"--auto", promptText}
	if !equalArgs(args, want) {
		t.Errorf("opencode auto: got %v, want %v", args, want)
	}

	// aider: --yes-always before --message flag before prompt.
	ai := &config.AgentConfig{SkipPermsFlag: "--yes-always", PromptFlag: "--message"}
	args = r.buildAgentArgs(ai, "aider", "implement", promptText)
	want = []string{"--yes-always", "--message", promptText}
	if !equalArgs(args, want) {
		t.Errorf("aider auto: got %v, want %v", args, want)
	}
}

func TestBuildBatchArgs(t *testing.T) {
	t.Setenv(forceManualEnv, "")
	prompt := "do the thing"

	tests := []struct {
		name   string
		cmd    string
		skip   string
		manual bool
		want   []string
	}{
		{"claude auto", "claude", "--dangerously-skip-permissions", false, []string{"--dangerously-skip-permissions", prompt}},
		{"claude manual", "claude", "--dangerously-skip-permissions", true, []string{prompt}},
		{"opencode auto", "opencode", "--auto", false, []string{"run", "--auto", prompt}},
		{"opencode manual", "opencode", "--auto", true, []string{"run", prompt}},
		{"aider auto", "aider", "--yes-always", false, []string{"run", "--yes-always", prompt}},
	}

	for _, tt := range tests {
		agentCfg := &config.AgentConfig{Cmd: tt.cmd, SkipPermsFlag: tt.skip}
		got := buildBatchArgs(agentCfg, prompt, tt.manual)
		if !equalArgs(got, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestPermissionArgs(t *testing.T) {
	agents := map[string]string{
		"claude-code": "--dangerously-skip-permissions",
		"opencode":    "--auto",
		"aider":       "--yes-always",
	}

	for name, flag := range agents {
		cfg := &config.AgentConfig{SkipPermsFlag: flag}

		// auto → skip flag emitted
		t.Setenv(forceManualEnv, "")
		if got := PermissionArgs(cfg, false); !equalArgs(got, []string{flag}) {
			t.Errorf("%s auto: got %v, want [%s]", name, got, flag)
		}

		// manual → nothing
		if got := PermissionArgs(cfg, true); len(got) != 0 {
			t.Errorf("%s manual: got %v, want empty", name, got)
		}

		// force-manual via env → nothing even with manual=false
		t.Setenv(forceManualEnv, "1")
		if got := PermissionArgs(cfg, false); len(got) != 0 {
			t.Errorf("%s force-manual: got %v, want empty", name, got)
		}
	}
}

func TestPermissionArgsEmptyFlag(t *testing.T) {
	t.Setenv(forceManualEnv, "")
	cfg := &config.AgentConfig{SkipPermsFlag: ""}
	if got := PermissionArgs(cfg, false); len(got) != 0 {
		t.Errorf("empty skip flag: got %v, want empty", got)
	}
}

func TestEffectiveMode(t *testing.T) {
	t.Setenv(forceManualEnv, "")
	if m := EffectiveMode(false); m != PermissionAuto {
		t.Errorf("auto: got %q", m)
	}
	if m := EffectiveMode(true); m != PermissionManual {
		t.Errorf("manual: got %q", m)
	}
	t.Setenv(forceManualEnv, "1")
	if m := EffectiveMode(false); m != PermissionForceManual {
		t.Errorf("force-manual: got %q", m)
	}
	if m := EffectiveMode(true); m != PermissionForceManual {
		t.Errorf("force-manual overrides manual: got %q", m)
	}
}

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
