package prompts

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
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

func PromptDir(stewardHome string) string {
	return filepath.Join(stewardHome, "prompts")
}

func PromptPath(stewardHome, phase string) string {
	return filepath.Join(PromptDir(stewardHome), phase+".txt")
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
