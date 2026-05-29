package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeConversation struct {
	Messages []claudeMessage `json:"messages"`
}

func captureClaudeLog(projectRoot string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	claudeDir := filepath.Join(home, ".claude", "projects")
	entries, err := os.ReadDir(claudeDir)
	if err != nil {
		return ""
	}

	var candidates []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidates = append(candidates, filepath.Join(claudeDir, e.Name()))
	}

	sort.Slice(candidates, func(i, j int) bool {
		infoI, _ := os.Stat(candidates[i])
		infoJ, _ := os.Stat(candidates[j])
		return infoI.ModTime().After(infoJ.ModTime())
	})

	for _, dir := range candidates {
		jsonlFiles, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
		if err != nil {
			continue
		}
		sort.Slice(jsonlFiles, func(i, j int) bool {
			infoI, _ := os.Stat(jsonlFiles[i])
			infoJ, _ := os.Stat(jsonlFiles[j])
			return infoI.ModTime().After(infoJ.ModTime())
		})
		if len(jsonlFiles) > 0 {
			return parseClaudeJSONL(jsonlFiles[0])
		}
	}

	return ""
}

func parseClaudeJSONL(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var parts []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg claudeMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
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
