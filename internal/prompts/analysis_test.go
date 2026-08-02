package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAnalysisTemplateEmbedded(t *testing.T) {
	for _, name := range []string{"go", "node", "docker-compose", "discovery-commands"} {
		content, err := LoadAnalysisTemplate("/nonexistent", name)
		if err != nil {
			t.Fatalf("LoadAnalysisTemplate(%q) returned error: %v", name, err)
		}
		if content == "" {
			t.Errorf("LoadAnalysisTemplate(%q) returned empty content", name)
		}
	}
}

func TestLoadAnalysisTemplateOverridePrecedence(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(AnalysisDir(home), 0755); err != nil {
		t.Fatal(err)
	}

	custom := "CUSTOM GO GUIDANCE"
	if err := os.WriteFile(AnalysisTemplatePath(home, "go"), []byte(custom), 0644); err != nil {
		t.Fatal(err)
	}

	content, err := LoadAnalysisTemplate(home, "go")
	if err != nil {
		t.Fatalf("LoadAnalysisTemplate with override returned error: %v", err)
	}
	if content != custom {
		t.Errorf("expected user override %q, got %q", custom, content)
	}
}

func TestLoadAnalysisTemplateOverrideCustomName(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(AnalysisDir(home), 0755); err != nil {
		t.Fatal(err)
	}

	custom := "CUSTOM"
	if err := os.WriteFile(AnalysisTemplatePath(home, "custom"), []byte(custom), 0644); err != nil {
		t.Fatal(err)
	}

	content, err := LoadAnalysisTemplate(home, "custom")
	if err != nil {
		t.Fatalf("user override without embedded default returned error: %v", err)
	}
	if content != custom {
		t.Errorf("expected %q, got %q", custom, content)
	}
}

func TestLoadAnalysisTemplateMissing(t *testing.T) {
	_, err := LoadAnalysisTemplate("/nonexistent", "does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing template")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("expected error to name the missing template, got: %v", err)
	}
	if !strings.Contains(err.Error(), "known") {
		t.Errorf("expected error to list known templates, got: %v", err)
	}
}

func TestLoadAnalysisTemplateEmptyName(t *testing.T) {
	if _, err := LoadAnalysisTemplate("/nonexistent", ""); err == nil {
		t.Fatal("expected error for empty template name")
	}
}

func TestLoadDiscoveryCommandsEmbedded(t *testing.T) {
	content, err := LoadDiscoveryCommands("/nonexistent")
	if err != nil {
		t.Fatalf("LoadDiscoveryCommands returned error: %v", err)
	}
	if content == "" {
		t.Fatal("expected non-empty discovery commands")
	}
	if !strings.Contains(content, "go mod graph") {
		t.Error("expected discovery commands to mention go mod graph")
	}
}

func TestLoadDiscoveryCommandsOverride(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(AnalysisDir(home), 0755); err != nil {
		t.Fatal(err)
	}

	custom := "CUSTOM COMMANDS"
	if err := os.WriteFile(AnalysisTemplatePath(home, "discovery-commands"), []byte(custom), 0644); err != nil {
		t.Fatal(err)
	}

	content, err := LoadDiscoveryCommands(home)
	if err != nil {
		t.Fatalf("LoadDiscoveryCommands with override returned error: %v", err)
	}
	if content != custom {
		t.Errorf("expected user override %q, got %q", custom, content)
	}
}

func TestAnalysisTemplateEmbedWiring(t *testing.T) {
	if len(knownAnalysisTemplates) == 0 {
		t.Fatal("expected at least one embedded analysis template")
	}

	hasDiscovery := false
	for _, name := range knownAnalysisTemplates {
		content, err := LoadAnalysisTemplate("/nonexistent", name)
		if err != nil {
			t.Fatalf("LoadAnalysisTemplate(%q) via embed wiring: %v", name, err)
		}
		if content == "" {
			t.Errorf("embedded template %q is empty", name)
		}
		if name == "discovery-commands" {
			hasDiscovery = true
		}
	}
	if !hasDiscovery {
		t.Error("expected discovery-commands.txt to be part of the embedded analysis/*.txt set")
	}
}

func TestAnalysisTemplateNotCopiedByWriteDefaultPrompts(t *testing.T) {
	home := t.TempDir()

	if err := WriteDefaultPrompts(home); err != nil {
		t.Fatalf("WriteDefaultPrompts returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, "prompts", "analysis")); !os.IsNotExist(err) {
		t.Error("analysis fragments should stay embedded and not be copied by WriteDefaultPrompts")
	}
}
