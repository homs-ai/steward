package fingerprint

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var BuildManifests = []string{
	"go.mod",
	"go.sum",
	"go.work",
	"go.work.sum",
	"package.json",
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"Cargo.toml",
	"Cargo.lock",
	"pyproject.toml",
	"requirements.txt",
	"setup.py",
	"setup.cfg",
	"Pipfile",
	"Pipfile.lock",
	"pom.xml",
	"build.gradle",
	"build.gradle.kts",
	"settings.gradle",
	"settings.gradle.kts",
	"*.csproj",
	"*.fsproj",
	"*.sln",
	"Dockerfile",
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

func ComputeFingerprint(projectRoot string) (string, error) {
	h := sha256.New()

	var files []string
	for _, pattern := range BuildManifests {
		if strings.Contains(pattern, "*") {
			matches, err := filepath.Glob(filepath.Join(projectRoot, pattern))
			if err != nil {
				continue
			}
			for _, m := range matches {
				rel, err := filepath.Rel(projectRoot, m)
				if err == nil {
					files = append(files, rel)
				}
			}
		} else {
			path := filepath.Join(projectRoot, pattern)
			if _, err := os.Stat(path); err == nil {
				rel, err := filepath.Rel(projectRoot, path)
				if err == nil {
					files = append(files, rel)
				}
			}
		}
	}

	sort.Strings(files)

	for _, file := range files {
		path := filepath.Join(projectRoot, file)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		fmt.Fprintf(h, "%s:", file)
		io.Copy(h, f)
		f.Close()
		fmt.Fprintf(h, "\n")
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func FingerprintChanged(projectRoot string, stored string) (bool, error) {
	current, err := ComputeFingerprint(projectRoot)
	if err != nil {
		return true, err
	}
	return current != stored, nil
}
