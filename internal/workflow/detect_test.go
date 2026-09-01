package workflow

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/k/steward/internal/prompts"
)

// writeTree creates a fixture tree under root from a map of relative paths to
// file contents.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// names returns the pattern names in a detection result, for compact assertions.
func names(patterns []DetectedPattern) []string {
	if len(patterns) == 0 {
		return nil
	}
	out := make([]string, 0, len(patterns))
	for _, p := range patterns {
		out = append(out, p.Name)
	}
	return out
}

func TestDetectBuildPatternsCuratedSet(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{
			name: "go module",
			files: map[string]string{
				"go.mod":    "module example.com/app\n\ngo 1.25\n",
				"go.sum":    "hash",
				"main.go":   "package main\n",
				"readme.md": "docs",
			},
			want: []string{"go"},
		},
		{
			name: "go workspace",
			files: map[string]string{
				"go.work": "go 1.25\n\nuse (\n\t./a\n\t./b\n)\n",
			},
			want: []string{"go-workspace"},
		},
		{
			name: "go module inside workspace still detects both",
			files: map[string]string{
				"go.mod":  "module example.com/root\n",
				"go.work": "go 1.25\n\nuse (\n\t.\n)\n",
			},
			want: []string{"go", "go-workspace"},
		},
		{
			name: "node project",
			files: map[string]string{
				"package.json": `{"name":"app","scripts":{"build":"tsc"}}`,
			},
			want: []string{"node"},
		},
		{
			name: "rust crate",
			files: map[string]string{
				"Cargo.toml":  `[package]\nname = "app"\n`,
				"src/main.rs": "fn main() {}\n",
			},
			want: []string{"rust"},
		},
		{
			name: "docker-compose yml",
			files: map[string]string{
				"docker-compose.yml": "services:\n  web:\n    image: nginx\n",
			},
			want: []string{"docker-compose"},
		},
		{
			name: "docker-compose yaml",
			files: map[string]string{
				"docker-compose.yaml": "services:\n  web:\n    image: nginx\n",
			},
			want: []string{"docker-compose"},
		},
		{
			name: "compose modern naming",
			files: map[string]string{
				"compose.yaml": "services:\n  web:\n    image: nginx\n",
			},
			want: []string{"docker-compose"},
		},
		{
			name: "dockerfile",
			files: map[string]string{
				"Dockerfile": "FROM golang:1.25\n",
			},
			want: []string{"dockerfile"},
		},
		{
			name: "dockerfile variant",
			files: map[string]string{
				"Dockerfile.dev": "FROM node:22\n",
			},
			want: []string{"dockerfile"},
		},
		{
			name: "ci workflow yml",
			files: map[string]string{
				".github/workflows/ci.yml": "name: CI\n",
			},
			want: []string{"ci"},
		},
		{
			name: "ci workflow yaml",
			files: map[string]string{
				".github/workflows/release.yaml": "name: Release\n",
			},
			want: []string{"ci"},
		},
		{
			name: "grpc proto at root",
			files: map[string]string{
				"service.proto": "syntax = \"proto3\";\n",
			},
			want: []string{"grpc"},
		},
		{
			name: "grpc proto in nested dirs",
			files: map[string]string{
				"api/v1/users.proto":     "syntax = \"proto3\";\n",
				"api/v1/orders.proto":    "syntax = \"proto3\";\n",
				"pkg/grpc/service.proto": "syntax = \"proto3\";\n",
			},
			want: []string{"grpc"},
		},
		{
			name: "python",
			files: map[string]string{
				"pyproject.toml": "[project]\nname = \"app\"\n",
			},
			want: []string{"python"},
		},
		{
			name: "java maven",
			files: map[string]string{
				"pom.xml": "<project/>\n",
			},
			want: []string{"java"},
		},
		{
			name: "dotnet nested project",
			files: map[string]string{
				"src/App/App.csproj": "<Project Sdk=\"Microsoft.NET.Sdk\"/>\n",
			},
			want: []string{"dotnet"},
		},
		{
			name: "make",
			files: map[string]string{
				"Makefile": "build:\n\tgo build ./...\n",
			},
			want: []string{"make"},
		},
		{
			name: "full-stack monorepo",
			files: map[string]string{
				"go.mod":                    "module example.com/app\n",
				"package.json":              `{"name":"web"}`,
				"Dockerfile":                "FROM node:22\n",
				"docker-compose.yml":        "services: {}\n",
				"api/service.proto":         "syntax = \"proto3\";\n",
				".github/workflows/ci.yaml": "name: CI\n",
			},
			want: []string{"ci", "docker-compose", "dockerfile", "go", "grpc", "node"},
		},
		{
			name:  "empty project",
			files: map[string]string{},
			want:  nil,
		},
		{
			name: "unrelated files only",
			files: map[string]string{
				"README.md":  "docs",
				".gitignore": "*.log\n",
				"main.py":    "print('hi')\n",
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, tt.files)

			patterns, err := DetectBuildPatterns(t.TempDir(), root)
			if err != nil {
				t.Fatalf("DetectBuildPatterns returned error: %v", err)
			}
			if got := names(patterns); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("detected patterns = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectBuildPatternsTemplatePopulated(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod":     "module example.com/app\n",
		"Cargo.toml": "[package]\nname = \"app\"\n",
	})

	patterns, err := DetectBuildPatterns(t.TempDir(), root)
	if err != nil {
		t.Fatalf("DetectBuildPatterns returned error: %v", err)
	}
	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(patterns))
	}
	for _, p := range patterns {
		if p.Template == "" {
			t.Errorf("pattern %s: expected non-empty embedded template", p.Name)
		}
		if !strings.Contains(p.Template, p.File) {
			t.Errorf("pattern %s: template should reference matched file %q", p.Name, p.File)
		}
	}
}

func TestDetectBuildPatternsTemplateOverride(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(prompts.AnalysisDir(home), 0755); err != nil {
		t.Fatal(err)
	}
	custom := "CUSTOM GO GUIDANCE"
	if err := os.WriteFile(prompts.AnalysisTemplatePath(home, "go"), []byte(custom), 0644); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	writeTree(t, root, map[string]string{"go.mod": "module example.com/app\n"})

	patterns, err := DetectBuildPatterns(home, root)
	if err != nil {
		t.Fatalf("DetectBuildPatterns returned error: %v", err)
	}
	if len(patterns) != 1 || patterns[0].Name != "go" {
		t.Fatalf("expected go pattern, got %+v", patterns)
	}
	if patterns[0].Template != custom {
		t.Errorf("expected user override template, got %q", patterns[0].Template)
	}
}

func TestDetectBuildPatternsYmlYamlPrecedence(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"docker-compose.yml":  "services: {}\n",
		"docker-compose.yaml": "services: {}\n",
	})

	patterns, err := DetectBuildPatterns(t.TempDir(), root)
	if err != nil {
		t.Fatalf("DetectBuildPatterns returned error: %v", err)
	}
	if len(patterns) != 1 {
		t.Fatalf("expected a single docker-compose pattern, got %+v", names(patterns))
	}
	if patterns[0].Name != "docker-compose" {
		t.Errorf("expected docker-compose pattern, got %q", patterns[0].Name)
	}
	if patterns[0].File != "docker-compose.yml" {
		t.Errorf("expected docker-compose.yml to win over .yaml, got %q", patterns[0].File)
	}
}

func TestDetectBuildPatternsGlobNestedDirs(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		wantName string
		wantFile string
	}{
		{
			name: "grpc first match in lexical order",
			files: map[string]string{
				"a/service.proto":  "syntax = \"proto3\";\n",
				"b/c/deeper.proto": "syntax = \"proto3\";\n",
				"z/proto/zz.proto": "syntax = \"proto3\";\n",
			},
			wantName: "grpc",
			wantFile: "a/service.proto",
		},
		{
			name: "ci workflow glob stays in workflows dir",
			files: map[string]string{
				".github/workflows/ci.yml":      "name: CI\n",
				".github/not-workflows/x.yml":   "name: no\n",
				".github/workflows/sub/out.yml": "name: nested\n",
			},
			wantName: "ci",
			wantFile: ".github/workflows/ci.yml",
		},
		{
			name: "dotnet glob finds nested project",
			files: map[string]string{
				"src/api/Api.csproj": "<Project/>\n",
				"src/tests/app.sln":  "sln\n",
			},
			wantName: "dotnet",
			wantFile: "src/api/Api.csproj",
		},
		{
			name: "nested package.json does not trigger node",
			files: map[string]string{
				"services/web/package.json": `{"name":"web"}`,
			},
			wantName: "",
			wantFile: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, tt.files)

			patterns, err := DetectBuildPatterns(t.TempDir(), root)
			if err != nil {
				t.Fatalf("DetectBuildPatterns returned error: %v", err)
			}
			if tt.wantName == "" {
				if len(patterns) != 0 {
					t.Errorf("expected no patterns, got %+v", names(patterns))
				}
				return
			}
			if len(patterns) != 1 {
				t.Fatalf("expected exactly one pattern, got %+v", names(patterns))
			}
			if patterns[0].Name != tt.wantName {
				t.Errorf("expected pattern %q, got %q", tt.wantName, patterns[0].Name)
			}
			if filepath.FromSlash(patterns[0].File) != tt.wantFile {
				t.Errorf("expected file %q, got %q", tt.wantFile, patterns[0].File)
			}
		})
	}
}

func TestDetectBuildPatternsPrunesDependencyDirs(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		wantName string
	}{
		{
			name: "vendored protos are ignored",
			files: map[string]string{
				"vendor/upstream/api.proto": "syntax = \"proto3\";\n",
				"node_modules/pkg/x.proto":  "syntax = \"proto3\";\n",
				".git/objects/y.proto":      "syntax = \"proto3\";\n",
			},
			wantName: "",
		},
		{
			name: "own protos still found alongside vendored ones",
			files: map[string]string{
				"api/service.proto":         "syntax = \"proto3\";\n",
				"vendor/upstream/api.proto": "syntax = \"proto3\";\n",
			},
			wantName: "grpc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, tt.files)

			patterns, err := DetectBuildPatterns(t.TempDir(), root)
			if err != nil {
				t.Fatalf("DetectBuildPatterns returned error: %v", err)
			}
			if tt.wantName == "" {
				if len(patterns) != 0 {
					t.Errorf("expected no patterns, got %+v", names(patterns))
				}
				return
			}
			if len(patterns) != 1 || patterns[0].Name != tt.wantName {
				t.Errorf("expected %q pattern, got %+v", tt.wantName, names(patterns))
			}
		})
	}
}

func TestDetectBuildPatternsErrors(t *testing.T) {
	t.Run("missing root", func(t *testing.T) {
		if _, err := DetectBuildPatterns(t.TempDir(), filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
			t.Fatal("expected error for missing project root")
		}
	})

	t.Run("root is a file", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "go.mod")
		if err := os.WriteFile(file, []byte("module x\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := DetectBuildPatterns(t.TempDir(), file); err == nil {
			t.Fatal("expected error when project root is not a directory")
		}
	})
}
