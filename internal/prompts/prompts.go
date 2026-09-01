package prompts

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed brainstorm.txt
var brainstormDefault string

//go:embed research.txt
var researchDefault string

//go:embed analysis.txt
var analysisDefault string

//go:embed implement.txt
var implementDefault string

//go:embed test.txt
var testDefault string

var defaultPrompts = map[string]string{
	"brainstorm": brainstormDefault,
	"research":   researchDefault,
	"analysis":   analysisDefault,
	"implement":  implementDefault,
	"test":       testDefault,
}

//go:embed analysis/*.txt
var analysisTemplatesFS embed.FS

//go:embed analysis/discovery-commands.txt
var discoveryCommandsDefault string

// knownAnalysisTemplates is the sorted list of build-pattern template names
// derived from the embedded analysis/*.txt set, used in error messages.
var knownAnalysisTemplates []string

func init() {
	entries, err := fs.ReadDir(analysisTemplatesFS, "analysis")
	if err != nil {
		panic(fmt.Sprintf("read embedded analysis templates: %v", err))
	}
	for _, entry := range entries {
		knownAnalysisTemplates = append(knownAnalysisTemplates, strings.TrimSuffix(entry.Name(), ".txt"))
	}
	sort.Strings(knownAnalysisTemplates)
}

func PromptDir(stewardHome string) string {
	return filepath.Join(stewardHome, "prompts")
}

func PromptPath(stewardHome, phase string) string {
	return filepath.Join(PromptDir(stewardHome), phase+".txt")
}

// AnalysisDir returns the user-overridable directory for build-pattern analysis
// templates: <stewardHome>/prompts/analysis. Analysis fragments are embedded by
// default; a file placed here for a given template name takes precedence.
func AnalysisDir(stewardHome string) string {
	return filepath.Join(PromptDir(stewardHome), "analysis")
}

// AnalysisTemplatePath returns the on-disk path for the named analysis template.
func AnalysisTemplatePath(stewardHome, name string) string {
	return filepath.Join(AnalysisDir(stewardHome), name+".txt")
}

// LoadAnalysisTemplate returns the build-specific analysis guidance fragment
// for the named pattern (e.g. "go", "node", "docker-compose"). A user override
// at <stewardHome>/prompts/analysis/<name>.txt wins over the embedded default;
// an unknown name returns an error that lists the known templates.
func LoadAnalysisTemplate(stewardHome, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("analysis template name must not be empty")
	}
	path := AnalysisTemplatePath(stewardHome, name)
	if data, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(data)), nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read analysis template %s: %w", path, err)
	}
	data, err := analysisTemplatesFS.ReadFile("analysis/" + name + ".txt")
	if err != nil {
		return "", fmt.Errorf("unknown analysis template: %s (known: %s)", name, strings.Join(knownAnalysisTemplates, ", "))
	}
	return strings.TrimSpace(string(data)), nil
}

// LoadDiscoveryCommands returns the universal discovery-command list injected
// into the research prompt. A user override at
// <stewardHome>/prompts/analysis/discovery-commands.txt wins over the embedded
// default.
func LoadDiscoveryCommands(stewardHome string) (string, error) {
	path := AnalysisTemplatePath(stewardHome, "discovery-commands")
	if data, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(data)), nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read discovery commands %s: %w", path, err)
	}
	return strings.TrimSpace(discoveryCommandsDefault), nil
}

func PromptForPhase(stewardHome, phase string) (string, error) {
	path := PromptPath(stewardHome, phase)
	data, err := os.ReadFile(path)
	if err == nil {
		return string(data), nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("read prompt %s: %w", path, err)
	}
	def, ok := defaultPrompts[phase]
	if !ok {
		return "", fmt.Errorf("unknown phase: %s", phase)
	}
	return def, nil
}

func WriteDefaultPrompts(stewardHome string) error {
	dir := PromptDir(stewardHome)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create prompt dir: %w", err)
	}
	for phase, content := range defaultPrompts {
		path := PromptPath(stewardHome, phase)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(strings.TrimSpace(content)), 0644); err != nil {
				return fmt.Errorf("write prompt %s: %w", phase, err)
			}
		}
	}
	return nil
}
