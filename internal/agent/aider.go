package agent

import (
	"fmt"
	"os"
	"path/filepath"
)

func captureAiderLog(projectRoot string) string {
	if projectRoot == "" {
		return ""
	}

	candidates := []string{
		filepath.Join(projectRoot, ".aider.chat.history.md"),
		filepath.Join(projectRoot, ".aider.chat.history"),
		filepath.Join(projectRoot, "aider_chat_history.md"),
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil {
			content := string(data)
			if len(content) > 2000 {
				content = content[:2000] + fmt.Sprintf("\n[...truncated, full length: %d chars]", len(content))
			}
			return content
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	globalFile := filepath.Join(home, ".aider.chat.history.md")
	data, err := os.ReadFile(globalFile)
	if err != nil {
		return ""
	}
	content := string(data)
	if len(content) > 2000 {
		content = content[:2000] + fmt.Sprintf("\n[...truncated, full length: %d chars]", len(content))
	}
	return content
}
