package agent

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type TmuxBackend struct {
	sessionName string
	logDir      string
	cmd         *exec.Cmd
	output      bytes.Buffer
}

func NewTmuxBackend(logDir string) *TmuxBackend {
	return &TmuxBackend{
		sessionName: fmt.Sprintf("steward-%d", time.Now().Unix()),
		logDir:      logDir,
	}
}

func (b *TmuxBackend) Start(cmd *exec.Cmd) error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux not found in PATH, fall back to pty backend")
	}

	args := []string{
		"new-session", "-d", "-s", b.sessionName,
		"-x", "120", "-y", "40",
	}

	startCmd := exec.Command("tmux", args...)
	if err := startCmd.Run(); err != nil {
		return fmt.Errorf("tmux new-session: %w", err)
	}

	sendArgs := []string{"send-keys", "-t", b.sessionName}
	sendArgs = append(sendArgs, cmd.Args...)
	sendArgs = append(sendArgs, "Enter")
	sendCmd := exec.Command("tmux", sendArgs...)
	if err := sendCmd.Run(); err != nil {
		b.cleanup()
		return fmt.Errorf("tmux send-keys: %w", err)
	}

	attachCmd := exec.Command("tmux", "attach-session", "-t", b.sessionName)
	attachCmd.Stdin = os.Stdin
	attachCmd.Stdout = os.Stdout
	attachCmd.Stderr = os.Stderr
	b.cmd = attachCmd

	if err := attachCmd.Start(); err != nil {
		b.cleanup()
		return fmt.Errorf("tmux attach: %w", err)
	}

	go func() {
		attachCmd.Wait()
		b.capturePaneOutput()
	}()

	return nil
}

func (b *TmuxBackend) Wait() error {
	if b.cmd != nil {
		return b.cmd.Wait()
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeout := time.After(24 * time.Hour)

	for {
		select {
		case <-ticker.C:
			hasSession := exec.Command("tmux", "has-session", "-t", b.sessionName)
			if hasSession.Run() != nil {
				b.capturePaneOutput()
				return nil
			}
		case <-timeout:
			return fmt.Errorf("tmux session timed out after 24h")
		}
	}
}

func (b *TmuxBackend) Signal(sig os.Signal) error {
	return exec.Command("tmux", "send-keys", "-t", b.sessionName, "C-c").Run()
}

func (b *TmuxBackend) CapturedOutput() string {
	return b.output.String()
}

func (b *TmuxBackend) Close() error {
	b.cleanup()
	return nil
}

func (b *TmuxBackend) Stdin() *os.File {
	return nil
}

func (b *TmuxBackend) cleanup() {
	exec.Command("tmux", "kill-session", "-t", b.sessionName).Run()
}

func (b *TmuxBackend) capturePaneOutput() {
	cmd := exec.Command("tmux", "capture-pane", "-t", b.sessionName, "-p", "-S", "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return
	}

	output := stdout.String()
	b.output.WriteString(output)

	if b.logDir != "" {
		logPath := filepath.Join(b.logDir, fmt.Sprintf("tmux_%s.log", b.sessionName))
		os.WriteFile(logPath, []byte(output), 0644)
	}
}
