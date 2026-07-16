package feature

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/k/steward/internal/config"
)

const StaleMarker = "<<<STALE>>>"

var RequiredFiles = []string{
	"req.md",
	"brainstorm.md",
	"research.md",
	"analysis.md",
	"diff.md",
	"report.md",
	"review.md",
	"test_scope.md",
	"test_artifacts.md",
	"test_report.md",
}

type Feature struct {
	Name         string
	Project      string
	Dir          string
	Created      time.Time
	BranchName   string
	BaseBranch   string
	WorktreePath string
}

func (f *Feature) DisplayName() string {
	if f.Project != "" {
		return f.Project + ":" + f.Name
	}
	return f.Name
}

func Init(cfg *config.Config, project, name string) (*Feature, error) {
	dir := filepath.Join(cfg.StewardHome, project, name)
	f := &Feature{
		Name:    name,
		Project: project,
		Dir:     dir,
		Created: time.Now(),
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create feature dir: %w", err)
	}

	for _, file := range RequiredFiles {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte{}, 0644); err != nil {
				return nil, fmt.Errorf("create %s: %w", file, err)
			}
		}
	}

	return f, nil
}

func Open(cfg *config.Config, project, name string) (*Feature, error) {
	dir := filepath.Join(cfg.StewardHome, project, name)
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("feature %q not found in project %q (run 'steward init %s' first)", name, project, name)
		}
		return nil, err
	}

	f := &Feature{
		Name:    name,
		Project: project,
		Dir:     dir,
		Created: info.ModTime(),
	}
	if err := f.LoadMetadata(); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *Feature) ReadFile(filename string) (string, error) {
	data, err := os.ReadFile(filepath.Join(f.Dir, filename))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *Feature) WriteFile(filename, content string) error {
	return os.WriteFile(filepath.Join(f.Dir, filename), []byte(content), 0644)
}

func (f *Feature) AppendFile(filename, content string) error {
	path := filepath.Join(f.Dir, filename)
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	newContent := string(current) + "\n" + content
	return os.WriteFile(path, []byte(newContent), 0644)
}

func (f *Feature) ReadAfterStale(filename string) (string, string, error) {
	content, err := f.ReadFile(filename)
	if err != nil {
		return "", "", err
	}

	idx := strings.LastIndex(content, StaleMarker)
	if idx == -1 {
		return content, content, nil
	}

	before := content[:idx]
	after := content[idx+len(StaleMarker):]
	return strings.TrimSpace(after), strings.TrimSpace(before), nil
}

func (f *Feature) WriteWithStale(filename, newContent string) error {
	content, err := f.ReadFile(filename)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		content = ""
	}

	updated := content
	if strings.TrimSpace(content) != "" {
		updated = content + "\n\n" + StaleMarker + "\n" + newContent
	} else {
		updated = newContent
	}

	return f.WriteFile(filename, updated)
}

func (f *Feature) TelemetryFile() string {
	return filepath.Join(f.Dir, "telemetry.yaml")
}

func (f *Feature) Exists(cfg *config.Config) bool {
	dir := filepath.Join(cfg.StewardHome, f.Project, f.Name)
	_, err := os.Stat(dir)
	return err == nil
}

func EnsureExists(cfg *config.Config, project, name string) (*Feature, error) {
	dir := filepath.Join(cfg.StewardHome, project, name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return Init(cfg, project, name)
	}
	return Open(cfg, project, name)
}

func listFeaturesInProject(cfg *config.Config, project string) ([]Feature, error) {
	projectDir := filepath.Join(cfg.StewardHome, project)
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var features []Feature
	for _, e := range entries {
		if e.IsDir() {
			info, _ := e.Info()
			features = append(features, Feature{
				Name:    e.Name(),
				Project: project,
				Dir:     filepath.Join(projectDir, e.Name()),
				Created: info.ModTime(),
			})
		}
	}
	return features, nil
}

func isFeatureDir(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	// Check if any child is a directory (would make this a project container, not a feature)
	for _, e := range entries {
		if e.IsDir() {
			return false
		}
	}
	// Check if any required files exist
	for _, f := range RequiredFiles {
		if _, err := os.Stat(filepath.Join(path, f)); err == nil {
			return true
		}
	}
	return false
}

func ListFeatures(cfg *config.Config, project string) ([]Feature, error) {
	if project != "" {
		return listFeaturesInProject(cfg, project)
	}

	entries, err := os.ReadDir(cfg.StewardHome)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var features []Feature
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		entryPath := filepath.Join(cfg.StewardHome, e.Name())

		if isFeatureDir(entryPath) {
			info, _ := e.Info()
			features = append(features, Feature{
				Name:    e.Name(),
				Project: "default",
				Dir:     entryPath,
				Created: info.ModTime(),
			})
		} else {
			projFeatures, err := listFeaturesInProject(cfg, e.Name())
			if err != nil {
				continue
			}
			features = append(features, projFeatures...)
		}
	}
	return features, nil
}

func PhaseFile(phase string) string {
	switch phase {
	case "brainstorm":
		return "brainstorm.md"
	case "research":
		return "research.md"
	case "analysis":
		return "analysis.md"
	case "implement":
		return "diff.md"
	case "test":
		return "test_report.md"
	default:
		return phase + ".md"
	}
}

func (f *Feature) FindPhase() string {
	order := []string{"brainstorm", "research", "analysis", "implement", "test"}
	for _, phase := range order {
		content, err := f.ReadFile(PhaseFile(phase))
		if err != nil || strings.TrimSpace(content) == "" {
			return phase
		}
	}
	return "complete"
}

func (f *Feature) LastActivity() time.Time {
	var latest time.Time
	filepath.Walk(f.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	return latest
}
