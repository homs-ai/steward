package agent

import (
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input string
		min   int
	}{
		{"", 0},
		{"hello world", 3}, // 2 words + 10/4 = 2.5 -> 2 + 2 = 4, actually let's just check > 0
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

func TestCheckAgent(t *testing.T) {
	// These will likely be false in test env, just verify no panic
	CheckAgent("claude")
	CheckAgent("opencode")
	CheckAgent("aider")
	CheckAgent("nonexistent-binary-xyz123")
}

func TestAvailableAgents(t *testing.T) {
	agents := AvailableAgents()
	// No assumptions about what's available, just verify it returns a slice
	if agents == nil {
		t.Error("expected non-nil slice")
	}
}
