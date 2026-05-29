package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type opencodeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type opencodeSession struct {
	Messages []opencodeMessage `json:"messages"`
}

func captureOpenCodeLog() string {
	xdgData := os.Getenv("XDG_DATA_HOME")
	if xdgData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		xdgData = filepath.Join(home, ".local", "share")
	}

	dbDir := filepath.Join(xdgData, "opencode")
	entries, err := os.ReadDir(dbDir)
	if err != nil {
		return ""
	}

	var jsonFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			jsonFiles = append(jsonFiles, filepath.Join(dbDir, e.Name()))
		}
	}

	sort.Slice(jsonFiles, func(i, j int) bool {
		infoI, _ := os.Stat(jsonFiles[i])
		infoJ, _ := os.Stat(jsonFiles[j])
		return infoI.ModTime().After(infoJ.ModTime())
	})

	if len(jsonFiles) == 0 {
		return ""
	}

	return parseOpenCodeJSON(jsonFiles[0])
}

func parseOpenCodeJSON(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var session opencodeSession
	if err := json.Unmarshal(data, &session); err != nil {
		return ""
	}

	var parts []string
	for _, msg := range session.Messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if len(content) > 500 {
			content = content[:500] + fmt.Sprintf("\n[...truncated, full length: %d chars]", len(content))
		}
		parts = append(parts, fmt.Sprintf("[%s] %s", msg.Role, content))
	}

	return strings.Join(parts, "\n\n")
}
