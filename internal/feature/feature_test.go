package feature

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k/steward/internal/config"
)

func setupTestFeature(t *testing.T) (*config.Config, string) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.DefaultConfig()
	cfg.StewardHome = filepath.Join(home, ".steward")
	return cfg, home
}

func TestInitFeature(t *testing.T) {
	cfg, _ := setupTestFeature(t)

	f, err := Init(cfg, "my-project", "test-feature")
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}
	if f.Name != "test-feature" {
		t.Errorf("expected name 'test-feature', got %q", f.Name)
	}
	if f.Project != "my-project" {
		t.Errorf("expected project 'my-project', got %q", f.Project)
	}
	if _, err := os.Stat(f.Dir); os.IsNotExist(err) {
		t.Error("feature directory was not created")
	}
	if !strings.HasSuffix(f.Dir, "/my-project/test-feature") && !strings.HasSuffix(f.Dir, "\\my-project\\test-feature") {
		t.Errorf("expected dir to end with my-project/test-feature, got %q", f.Dir)
	}

	for _, file := range RequiredFiles {
		path := filepath.Join(f.Dir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("required file %s was not created", file)
		}
	}
}

func TestInitOpenWithProject(t *testing.T) {
	cfg, _ := setupTestFeature(t)

	f, err := Init(cfg, "project-a", "feature-x")
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	f2, err := Open(cfg, "project-a", "feature-x")
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	if f2.Name != f.Name {
		t.Errorf("expected name %q, got %q", f.Name, f2.Name)
	}
	if f2.Project != "project-a" {
		t.Errorf("expected project 'project-a', got %q", f2.Project)
	}

	_, err = Open(cfg, "project-b", "feature-x")
	if err == nil {
		t.Error("expected error opening feature from wrong project")
	}
}

func TestOpenFeature(t *testing.T) {
	cfg, _ := setupTestFeature(t)

	f, err := Init(cfg, "my-project", "test-feature")
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	f2, err := Open(cfg, "my-project", "test-feature")
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	if f2.Name != f.Name {
		t.Errorf("expected name %q, got %q", f.Name, f2.Name)
	}

	_, err = Open(cfg, "my-project", "no-such-feature")
	if err == nil {
		t.Error("expected error opening non-existent feature")
	}
}

func TestReadWriteFile(t *testing.T) {
	cfg, _ := setupTestFeature(t)
	f, err := Init(cfg, "my-project", "test-feature")
	if err != nil {
		t.Fatal(err)
	}

	content := "# Test Content\nHello World"
	if err := f.WriteFile("req.md", content); err != nil {
		t.Fatalf("WriteFile() returned error: %v", err)
	}

	got, err := f.ReadFile("req.md")
	if err != nil {
		t.Fatalf("ReadFile() returned error: %v", err)
	}
	if got != content {
		t.Errorf("expected %q, got %q", content, got)
	}
}

func TestStaleMarker(t *testing.T) {
	cfg, _ := setupTestFeature(t)
	f, err := Init(cfg, "my-project", "test-feature")
	if err != nil {
		t.Fatal(err)
	}

	if err := f.WriteWithStale("brainstorm.md", "First round of ideas"); err != nil {
		t.Fatalf("WriteWithStale() returned error: %v", err)
	}

	if err := f.WriteWithStale("brainstorm.md", "Refined ideas"); err != nil {
		t.Fatalf("WriteWithStale() returned error: %v", err)
	}

	after, before, err := f.ReadAfterStale("brainstorm.md")
	if err != nil {
		t.Fatalf("ReadAfterStale() returned error: %v", err)
	}

	if after != "Refined ideas" {
		t.Errorf("expected 'Refined ideas', got %q", after)
	}
	if !strings.Contains(before, "First round of ideas") {
		t.Errorf("before content should contain first round, got %q", before)
	}
}

func TestFindPhase(t *testing.T) {
	cfg, _ := setupTestFeature(t)
	f, err := Init(cfg, "my-project", "test-feature")
	if err != nil {
		t.Fatal(err)
	}

	if phase := f.FindPhase(); phase != "brainstorm" {
		t.Errorf("expected phase 'brainstorm' for empty feature, got %q", phase)
	}

	f.WriteFile("brainstorm.md", "# Ideas")
	if phase := f.FindPhase(); phase != "research" {
		t.Errorf("expected phase 'research' after brainstorm, got %q", phase)
	}

	f.WriteFile("research.md", "# Research")
	if phase := f.FindPhase(); phase != "analysis" {
		t.Errorf("expected phase 'analysis' after research, got %q", phase)
	}

	f.WriteFile("analysis.md", "# Analysis")
	if phase := f.FindPhase(); phase != "implement" {
		t.Errorf("expected phase 'implement' after analysis, got %q", phase)
	}

	f.WriteFile("diff.md", "# Changes")
	if phase := f.FindPhase(); phase != "test" {
		t.Errorf("expected phase 'test' after implement, got %q", phase)
	}

	f.WriteFile("test_report.md", "# Tests")
	if phase := f.FindPhase(); phase != "complete" {
		t.Errorf("expected phase 'complete' after test, got %q", phase)
	}
}

func TestListFeaturesInProject(t *testing.T) {
	cfg, _ := setupTestFeature(t)

	features, err := ListFeatures(cfg, "my-project")
	if err != nil {
		t.Fatalf("ListFeatures() returned error: %v", err)
	}
	if len(features) != 0 {
		t.Errorf("expected 0 features, got %d", len(features))
	}

	Init(cfg, "my-project", "feature-a")
	Init(cfg, "my-project", "feature-b")

	features, err = ListFeatures(cfg, "my-project")
	if err != nil {
		t.Fatalf("ListFeatures() returned error: %v", err)
	}
	if len(features) != 2 {
		t.Errorf("expected 2 features, got %d", len(features))
	}
}

func TestListFeaturesAll(t *testing.T) {
	cfg, _ := setupTestFeature(t)

	Init(cfg, "my-project", "feature-a")
	Init(cfg, "other-project", "feature-b")

	features, err := ListFeatures(cfg, "")
	if err != nil {
		t.Fatalf("ListFeatures() returned error: %v", err)
	}
	if len(features) != 2 {
		t.Errorf("expected 2 features across all projects, got %d", len(features))
	}
}

func TestListFeaturesLegacyDefault(t *testing.T) {
	cfg, _ := setupTestFeature(t)

	// Create a legacy flat feature (no project scope, directly under StewardHome)
	dir := filepath.Join(cfg.StewardHome, "legacy-feature")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "req.md"), []byte("legacy"), 0644)

	features, err := ListFeatures(cfg, "")
	if err != nil {
		t.Fatalf("ListFeatures() returned error: %v", err)
	}
	if len(features) != 1 {
		t.Fatalf("expected 1 legacy feature, got %d", len(features))
	}
	if features[0].Name != "legacy-feature" {
		t.Errorf("expected name 'legacy-feature', got %q", features[0].Name)
	}
	if features[0].Project != "default" {
		t.Errorf("expected project 'default' for legacy feature, got %q", features[0].Project)
	}
}

func TestEnsureExists(t *testing.T) {
	cfg, _ := setupTestFeature(t)

	f, err := EnsureExists(cfg, "my-project", "new-feature")
	if err != nil {
		t.Fatalf("EnsureExists() returned error: %v", err)
	}
	if f.Name != "new-feature" {
		t.Errorf("expected name 'new-feature', got %q", f.Name)
	}
	if f.Project != "my-project" {
		t.Errorf("expected project 'my-project', got %q", f.Project)
	}

	f2, err := EnsureExists(cfg, "my-project", "new-feature")
	if err != nil {
		t.Fatalf("EnsureExists() returned error: %v", err)
	}
	if f2.Name != "new-feature" {
		t.Errorf("expected name 'new-feature' on re-open, got %q", f2.Name)
	}
}

func TestDisplayName(t *testing.T) {
	f := Feature{Name: "my-feature", Project: "my-project"}
	if f.DisplayName() != "my-project:my-feature" {
		t.Errorf("expected 'my-project:my-feature', got %q", f.DisplayName())
	}

	f2 := Feature{Name: "legacy"}
	if f2.DisplayName() != "legacy" {
		t.Errorf("expected 'legacy', got %q", f2.DisplayName())
	}
}

func TestPhaseFile(t *testing.T) {
	tests := []struct {
		phase string
		want  string
	}{
		{"brainstorm", "brainstorm.md"},
		{"research", "research.md"},
		{"analysis", "analysis.md"},
		{"implement", "diff.md"},
		{"test", "test_report.md"},
	}
	for _, tt := range tests {
		got := PhaseFile(tt.phase)
		if got != tt.want {
			t.Errorf("PhaseFile(%q) = %q, want %q", tt.phase, got, tt.want)
		}
	}
}

func TestPhaseFileDefault(t *testing.T) {
	got := PhaseFile("custom-phase")
	want := "custom-phase.md"
	if got != want {
		t.Errorf("PhaseFile(%q) = %q, want %q", "custom-phase", got, want)
	}
}
