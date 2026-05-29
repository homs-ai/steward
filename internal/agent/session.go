package agent

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/k/steward/internal/feature"
)

type SessionCapture struct {
	Feature          *feature.Feature
	Phase            string
	RawOutput        string
	GitDiff          string
	NewFiles         []string
	ConversationLogs string
	ProjectRoot      string
}

func CaptureSession(feat *feature.Feature, phase, rawOutput, projectRoot string) *SessionCapture {
	s := &SessionCapture{
		Feature:     feat,
		Phase:       phase,
		RawOutput:   rawOutput,
		ProjectRoot: projectRoot,
	}
	s.captureGitDiff()
	s.captureNewFiles()
	s.captureConversationLogs()
	return s
}

func (s *SessionCapture) captureGitDiff() {
	if s.ProjectRoot == "" {
		return
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = s.ProjectRoot
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		s.GitDiff = stdout.String()
	}
}

func (s *SessionCapture) captureNewFiles() {
	if s.ProjectRoot == "" {
		return
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = s.ProjectRoot
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		output := strings.TrimSpace(stdout.String())
		if output != "" {
			s.NewFiles = strings.Split(output, "\n")
		}
	}
}

func (s *SessionCapture) captureConversationLogs() {
	var logs []string

	if claudeLog := captureClaudeLog(s.ProjectRoot); claudeLog != "" {
		logs = append(logs, "=== Claude Conversation ===\n"+claudeLog)
	}
	if aiderLog := captureAiderLog(s.ProjectRoot); aiderLog != "" {
		logs = append(logs, "=== Aider Conversation ===\n"+aiderLog)
	}
	if opencodeLog := captureOpenCodeLog(); opencodeLog != "" {
		logs = append(logs, "=== OpenCode Conversation ===\n"+opencodeLog)
	}

	s.ConversationLogs = strings.Join(logs, "\n\n")
}

func (s *SessionCapture) SaveArtifacts() error {
	var parts []string

	parts = append(parts, "## Session Summary")
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("**Phase:** %s", s.Phase))
	parts = append(parts, fmt.Sprintf("**Output Size:** %d characters", len(s.RawOutput)))

	if s.GitDiff != "" {
		parts = append(parts, "")
		parts = append(parts, "## Changes (git diff HEAD)")
		parts = append(parts, "")
		parts = append(parts, "```diff")
		parts = append(parts, s.GitDiff)
		parts = append(parts, "```")
	}

	if len(s.NewFiles) > 0 {
		parts = append(parts, "")
		parts = append(parts, "## New Files")
		for _, f := range s.NewFiles {
			parts = append(parts, fmt.Sprintf("- %s", f))
		}
	}

	if s.ConversationLogs != "" {
		parts = append(parts, "")
		parts = append(parts, "## Conversation Logs")
		parts = append(parts, "")
		parts = append(parts, s.ConversationLogs)
	}

	artifact := strings.Join(parts, "\n")

	sessionFile := filepath.Join(s.Feature.Dir, fmt.Sprintf("%s_session.md", s.Phase))
	if err := os.MkdirAll(filepath.Dir(sessionFile), 0755); err != nil {
		return err
	}
	return os.WriteFile(sessionFile, []byte(artifact), 0644)
}

func (s *SessionCapture) BuildStructuredArtifact() string {
	var parts []string

	phaseDisplay := strings.ToUpper(s.Phase[:1]) + s.Phase[1:]
	parts = append(parts, fmt.Sprintf("# %s Phase Report", phaseDisplay))
	parts = append(parts, "")
	parts = append(parts, "## Key Insights")
	parts = append(parts, "")
	parts = append(parts, "(Auto-extracted insights would appear here)")
	parts = append(parts, "")
	parts = append(parts, "## Decisions")
	parts = append(parts, "")
	parts = append(parts, "(Key decisions made during this phase)")
	parts = append(parts, "")
	parts = append(parts, "## Recommendations")
	parts = append(parts, "")
	parts = append(parts, "(Recommendations for the next phase)")
	parts = append(parts, "")
	parts = append(parts, "## Open Questions")
	parts = append(parts, "")
	parts = append(parts, "(Unresolved questions that need attention)")
	parts = append(parts, "")

	if s.GitDiff != "" {
		parts = append(parts, "## Changes")
		parts = append(parts, "")
		parts = append(parts, "```diff")
		parts = append(parts, s.GitDiff)
		parts = append(parts, "```")
	}

	return strings.Join(parts, "\n")
}
