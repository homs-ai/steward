package workflow

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/k/steward/internal/prompts"
)

// DetectedPattern describes a build pattern found in a project tree. Name is
// the analysis-template name (e.g. "go", "docker-compose"); File is the path
// (relative to the project root) of the manifest that triggered the match; and
// Template is the build-specific guidance to inject into the research prompt.
type DetectedPattern struct {
	Name     string
	File     string
	Template string
}

// skipDirNames are directories pruned during recursive glob scans so vendored
// or generated dependency trees do not produce false positives.
var skipDirNames = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
}

// patternSpec is one entry in the curated detection set: the analysis-template
// name plus the file names and globs that trigger it. Exact file names are
// scoped to the project root; globs may recurse into nested directories.
type patternSpec struct {
	name  string
	files []string
	globs []string
}

// buildPatternSpecs is the curated set of build patterns detection understands,
// mirroring the analysis/*.txt templates. Order within a spec determines match
// precedence (e.g. docker-compose.yml wins over docker-compose.yaml).
var buildPatternSpecs = []patternSpec{
	{name: "go", files: []string{"go.mod"}},
	{name: "go-workspace", files: []string{"go.work"}},
	{name: "node", files: []string{"package.json"}},
	{name: "rust", files: []string{"Cargo.toml"}},
	{name: "python", files: []string{"pyproject.toml", "requirements.txt", "setup.py", "setup.cfg", "Pipfile"}},
	{name: "java", files: []string{"pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts"}},
	{name: "dotnet", globs: []string{"**/*.csproj", "**/*.fsproj", "**/*.sln"}},
	{name: "make", files: []string{"Makefile", "makefile", "GNUmakefile"}},
	{name: "dockerfile", globs: []string{"Dockerfile*"}},
	{name: "docker-compose", files: []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}},
	{name: "grpc", globs: []string{"**/*.proto"}},
	{name: "ci", globs: []string{".github/workflows/*.yml", ".github/workflows/*.yaml"}},
}

// DetectBuildPatterns scans projectRoot for the curated set of build patterns
// and returns one DetectedPattern per matched pattern, ordered by pattern name.
// Each pattern's Template is resolved through prompts.LoadAnalysisTemplate, so a
// user override under stewardHome wins over the embedded default. Exact file
// matches are scoped to the project root; glob patterns scan nested directories
// while pruning vendored dependency trees.
func DetectBuildPatterns(stewardHome, projectRoot string) ([]DetectedPattern, error) {
	matches, err := scanProject(projectRoot)
	if err != nil {
		return nil, err
	}

	patterns := make([]DetectedPattern, 0, len(matches))
	for _, m := range matches {
		template, err := prompts.LoadAnalysisTemplate(stewardHome, m.name)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, DetectedPattern{
			Name:     m.name,
			File:     m.file,
			Template: template,
		})
	}
	sort.Slice(patterns, func(i, j int) bool { return patterns[i].Name < patterns[j].Name })
	return patterns, nil
}

// patternMatch is an internal detection result before template loading.
type patternMatch struct {
	name string
	file string
}

// scanProject walks the tree once per spec and collects one match per pattern.
func scanProject(projectRoot string) ([]patternMatch, error) {
	root := filepath.Clean(projectRoot)
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &os.PathError{Op: "scan", Path: root, Err: os.ErrInvalid}
	}

	matches := make([]patternMatch, 0, len(buildPatternSpecs))
	for _, spec := range buildPatternSpecs {
		file, ok := firstFileMatch(root, spec)
		if !ok {
			continue
		}
		matches = append(matches, patternMatch{name: spec.name, file: file})
	}
	return matches, nil
}

// firstFileMatch returns the first file (relative to root) that triggers spec,
// checking exact file names before glob patterns.
func firstFileMatch(root string, spec patternSpec) (string, bool) {
	for _, name := range spec.files {
		if isFile(filepath.Join(root, name)) {
			return name, true
		}
	}
	for _, glob := range spec.globs {
		if file, ok := firstGlobMatch(root, glob); ok {
			return file, true
		}
	}
	return "", false
}

// isFile reports whether path exists and is a regular file.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// firstGlobMatch returns the first file under root whose path relative to root
// matches glob. Walk order is lexical, so the result is deterministic; vendored
// dependency directories are pruned.
func firstGlobMatch(root, glob string) (string, bool) {
	found := ""
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && skipDirNames[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		if matchesGlob(filepath.ToSlash(rel), glob) {
			found = filepath.ToSlash(rel)
			return fs.SkipAll
		}
		return nil
	})
	return found, found != ""
}

// matchesGlob reports whether a slash-separated relative path matches glob. A
// leading "**/" matches zero or more directory components; everything else
// follows filepath.Match semantics.
func matchesGlob(rel, glob string) bool {
	if strings.HasPrefix(glob, "**/") {
		return matchesTail(rel, strings.TrimPrefix(glob, "**/"))
	}
	ok, err := filepath.Match(glob, rel)
	return err == nil && ok
}

// matchesTail reports whether rel's trailing path components match tail, where
// each tail component follows filepath.Match semantics.
func matchesTail(rel, tail string) bool {
	relParts := strings.Split(rel, "/")
	tailParts := strings.Split(tail, "/")
	if len(relParts) < len(tailParts) {
		return false
	}
	offset := len(relParts) - len(tailParts)
	for i, tp := range tailParts {
		ok, err := filepath.Match(tp, relParts[offset+i])
		if err != nil || !ok {
			return false
		}
	}
	return true
}
