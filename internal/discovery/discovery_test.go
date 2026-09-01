package discovery

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCommandSetRunBasic(t *testing.T) {
	cs := NewCommandSet(t.TempDir(), 5*time.Second)
	result := cs.Run(context.Background(), "echo hello")
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Output != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", result.Output)
	}
}

func TestCommandSetRunWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("found"), 0644)

	cs := NewCommandSet(dir, 5*time.Second)
	result := cs.Run(context.Background(), "cat marker.txt")
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Output != "found" {
		t.Errorf("expected 'found', got %q", result.Output)
	}
}

func TestCommandSetRunTimeout(t *testing.T) {
	cs := NewCommandSet(t.TempDir(), 50*time.Millisecond)
	result := cs.Run(context.Background(), "sleep 5")
	if !result.Timeout {
		t.Error("expected timeout to be true")
	}
}

func TestCommandSetRunError(t *testing.T) {
	cs := NewCommandSet(t.TempDir(), 5*time.Second)
	result := cs.Run(context.Background(), "false")
	if result.Error == "" {
		t.Error("expected error for failing command")
	}
}

func TestCommandSetRunAll(t *testing.T) {
	cs := NewCommandSet(t.TempDir(), 5*time.Second)
	results := cs.RunAll(context.Background(), []string{"echo a", "echo b"})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Output != "a\n" {
		t.Errorf("expected 'a\\n', got %q", results[0].Output)
	}
	if results[1].Output != "b\n" {
		t.Errorf("expected 'b\\n', got %q", results[1].Output)
	}
}

func TestFilterByEcosystem(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		commands []string
		wantLen  int
	}{
		{
			name:     "go pattern",
			patterns: []string{"go"},
			commands: StandardCommands,
			wantLen:  3,
		},
		{
			name:     "no matching patterns returns all",
			patterns: []string{"unknown"},
			commands: StandardCommands,
			wantLen:  len(StandardCommands),
		},
		{
			name:     "empty patterns returns all",
			patterns: []string{},
			commands: StandardCommands,
			wantLen:  len(StandardCommands),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterByEcosystem(tt.patterns, tt.commands)
			if len(filtered) != tt.wantLen {
				t.Errorf("expected %d commands, got %d", tt.wantLen, len(filtered))
			}
		})
	}
}

func TestResultsToText(t *testing.T) {
	results := []Result{
		{Command: "echo ok", Output: "ok\n"},
		{Command: "timeout cmd", Timeout: true},
		{Command: "failing", Error: "exit status 1"},
	}

	text := ResultsToText(results)
	if text == "" {
		t.Error("expected non-empty text")
	}
}
