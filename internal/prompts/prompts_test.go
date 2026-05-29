package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptForPhaseBrainstorm(t *testing.T) {
	prompt, err := PromptForPhase("/nonexistent", "brainstorm")
	if err != nil {
		t.Fatalf("PromptForPhase(brainstorm) returned error: %v", err)
	}
	if prompt == "" {
		t.Fatal("expected non-empty brainstorm prompt")
	}
	if !strings.Contains(prompt, "divergent thinking") {
		t.Error("expected brainstorm prompt to mention divergent thinking")
	}
	if !strings.Contains(prompt, "idea generation") {
		t.Error("expected brainstorm prompt to mention idea generation")
	}
	if !strings.Contains(prompt, "Wild or innovative ideas") {
		t.Error("expected brainstorm prompt to mention wild ideas")
	}
}

func TestPromptForPhaseResearch(t *testing.T) {
	prompt, err := PromptForPhase("/nonexistent", "research")
	if err != nil {
		t.Fatalf("PromptForPhase(research) returned error: %v", err)
	}
	if prompt == "" {
		t.Fatal("expected non-empty research prompt")
	}
}

func TestPromptForPhaseAnalysis(t *testing.T) {
	prompt, err := PromptForPhase("/nonexistent", "analysis")
	if err != nil {
		t.Fatalf("PromptForPhase(analysis) returned error: %v", err)
	}
	if prompt == "" {
		t.Fatal("expected non-empty analysis prompt")
	}
}

func TestPromptForPhaseImplement(t *testing.T) {
	prompt, err := PromptForPhase("/nonexistent", "implement")
	if err != nil {
		t.Fatalf("PromptForPhase(implement) returned error: %v", err)
	}
	if prompt == "" {
		t.Fatal("expected non-empty implement prompt")
	}
}

func TestPromptForPhaseTest(t *testing.T) {
	prompt, err := PromptForPhase("/nonexistent", "test")
	if err != nil {
		t.Fatalf("PromptForPhase(test) returned error: %v", err)
	}
	if prompt == "" {
		t.Fatal("expected non-empty test prompt")
	}
}

func TestPromptForPhaseUnknown(t *testing.T) {
	_, err := PromptForPhase("/nonexistent", "nonexistent-phase")
	if err == nil {
		t.Fatal("expected error for unknown phase")
	}
}

func TestPromptForPhaseCustom(t *testing.T) {
	home := t.TempDir()

	promptDir := filepath.Join(home, "prompts")
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		t.Fatal(err)
	}

	customContent := "This is a custom brainstorm prompt override"
	if err := os.WriteFile(filepath.Join(promptDir, "brainstorm.txt"), []byte(customContent), 0644); err != nil {
		t.Fatal(err)
	}

	prompt, err := PromptForPhase(home, "brainstorm")
	if err != nil {
		t.Fatalf("PromptForPhase with custom prompt returned error: %v", err)
	}
	if prompt != customContent {
		t.Errorf("expected custom content %q, got %q", customContent, prompt)
	}
}

func TestPromptDir(t *testing.T) {
	dir := PromptDir("/tmp/test-steward")
	expected := "/tmp/test-steward/prompts"
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

func TestPromptPath(t *testing.T) {
	path := PromptPath("/tmp/test-steward", "brainstorm")
	expected := "/tmp/test-steward/prompts/brainstorm.txt"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestWriteDefaultPrompts(t *testing.T) {
	home := t.TempDir()

	if err := WriteDefaultPrompts(home); err != nil {
		t.Fatalf("WriteDefaultPrompts() returned error: %v", err)
	}

	expectedPhases := []string{"brainstorm", "research", "analysis", "implement", "test"}
	for _, phase := range expectedPhases {
		path := filepath.Join(home, "prompts", phase+".txt")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("prompt file %s was not created", path)
		}
	}
}

func TestWriteDefaultPromptsIdempotent(t *testing.T) {
	home := t.TempDir()

	if err := WriteDefaultPrompts(home); err != nil {
		t.Fatal(err)
	}

	customContent := "CUSTOM OVERRIDE - should NOT be overwritten"
	promptDir := filepath.Join(home, "prompts")
	os.WriteFile(filepath.Join(promptDir, "brainstorm.txt"), []byte(customContent), 0644)

	if err := WriteDefaultPrompts(home); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(promptDir, "brainstorm.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != customContent {
		t.Errorf("WriteDefaultPrompts overwrote existing file: got %q, want %q", string(data), customContent)
	}
}
