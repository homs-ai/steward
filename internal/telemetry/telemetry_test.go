package telemetry

import (
	"os"
	"path/filepath"
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

func TestLoadEmptyTelemetry(t *testing.T) {
	_, f := setupTest(t)

	ft, err := Load(f)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if ft.Feature != "test-feature" {
		t.Errorf("expected feature 'test-feature', got %q", ft.Feature)
	}
	if len(ft.Phases) != 0 {
		t.Errorf("expected 0 phases, got %d", len(ft.Phases))
	}
}

func TestRecordPhaseStart(t *testing.T) {
	_, f := setupTest(t)

	if err := RecordPhaseStart(f, "brainstorm", "claude-code", "auto"); err != nil {
		t.Fatalf("RecordPhaseStart() returned error: %v", err)
	}

	ft, err := Load(f)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	p := ft.Phases["brainstorm"]
	if p == nil {
		t.Fatal("expected brainstorm phase telemetry")
	}
	if p.Agent != "claude-code" {
		t.Errorf("expected agent 'claude-code', got %q", p.Agent)
	}
	if p.StartedAt == "" {
		t.Error("expected started_at to be set")
	}
}

func TestRecordPhaseEnd(t *testing.T) {
	_, f := setupTest(t)

	if err := RecordPhaseStart(f, "implement", "aider", "auto"); err != nil {
		t.Fatalf("RecordPhaseStart() returned error: %v", err)
	}

	if err := RecordPhaseEnd(f, "implement", 15000, 5000, 0, 0, ""); err != nil {
		t.Fatalf("RecordPhaseEnd() returned error: %v", err)
	}

	ft, err := Load(f)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	p := ft.Phases["implement"]
	if p.TokensIn != 15000 {
		t.Errorf("expected 15000 tokens in, got %d", p.TokensIn)
	}
	if p.TokensOut != 5000 {
		t.Errorf("expected 5000 tokens out, got %d", p.TokensOut)
	}
	if p.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", p.ExitCode)
	}
	if p.Iterations != 1 {
		t.Errorf("expected 1 iteration, got %d", p.Iterations)
	}
}

func TestRecordPhaseEndWithError(t *testing.T) {
	_, f := setupTest(t)

	RecordPhaseStart(f, "test", "claude-code", "auto")
	if err := RecordPhaseEnd(f, "test", 1000, 500, 1, 2, "agent crashed"); err != nil {
		t.Fatalf("RecordPhaseEnd() returned error: %v", err)
	}

	ft, _ := Load(f)
	p := ft.Phases["test"]
	if p.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", p.ExitCode)
	}
	if p.Retries != 2 {
		t.Errorf("expected 2 retries, got %d", p.Retries)
	}
	if p.Error != "agent crashed" {
		t.Errorf("expected error 'agent crashed', got %q", p.Error)
	}
}

func TestRecordRating(t *testing.T) {
	_, f := setupTest(t)

	RecordPhaseStart(f, "brainstorm", "claude-code", "auto")
	RecordPhaseEnd(f, "brainstorm", 5000, 2000, 0, 0, "")

	if err := RecordRating(f, "brainstorm", 4); err != nil {
		t.Fatalf("RecordRating() returned error: %v", err)
	}

	ft, _ := Load(f)
	if ft.Phases["brainstorm"].HumanRating != 4 {
		t.Errorf("expected rating 4, got %d", ft.Phases["brainstorm"].HumanRating)
	}
}

func TestEstimateCost(t *testing.T) {
	cfg, _ := setupTest(t)

	// claude-code: $0.003/1k in, $0.015/1k out
	cost := EstimateCost(cfg, "claude-code", 10000, 5000)
	expected := (10000.0/1000.0)*0.003 + (5000.0/1000.0)*0.015
	if cost != expected {
		t.Errorf("expected cost %f, got %f", expected, cost)
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{500, "500"},
		{1500, "1.5k"},
		{1500000, "1.5M"},
	}
	for _, tt := range tests {
		got := FormatTokens(tt.input)
		if got != tt.want {
			t.Errorf("FormatTokens(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatCost(t *testing.T) {
	got := FormatCost(3.95)
	if got != "$3.95" {
		t.Errorf("expected '$3.95', got %q", got)
	}

	got = FormatCost(0.5)
	if got != "$0.50" {
		t.Errorf("expected '$0.50', got %q", got)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{30, "30s"},
		{120, "2m"},
		{3661, "1h1m"},
	}
	for _, tt := range tests {
		got := FormatDuration(tt.input)
		if got != tt.want {
			t.Errorf("FormatDuration(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSaveTelemetryFile(t *testing.T) {
	_, f := setupTest(t)

	ft := &FeatureTelemetry{
		Feature: f.Name,
		Phases: map[string]*PhaseTelemetry{
			"brainstorm": {
				Agent:     "claude-code",
				TokensIn:  5000,
				TokensOut: 2000,
			},
		},
	}

	if err := Save(f, ft); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	if _, err := os.Stat(f.TelemetryFile()); os.IsNotExist(err) {
		t.Error("telemetry file was not created")
	}
}

func TestAggregateAgentsEmpty(t *testing.T) {
	cfg, _ := setupTest(t)

	summaries, err := AggregateAgents(cfg)
	if err != nil {
		t.Fatalf("AggregateAgents() returned error: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("expected 0 summaries, got %d", len(summaries))
	}
}
