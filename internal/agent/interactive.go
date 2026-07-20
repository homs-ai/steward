package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/k/steward/internal/config"
	"github.com/k/steward/internal/feature"
	"github.com/k/steward/internal/telemetry"
)

type InteractiveBackend interface {
	Start(cmd *exec.Cmd) error
	Wait() error
	Signal(sig os.Signal) error
	CapturedOutput() string
	Close() error
}

type InteractiveRunner struct {
	Config      *config.Config
	Backend     InteractiveBackend
	ProjectRoot string
	LogDir      string
	// Manual, when true, requests manual permission mode (no skip flag). The
	// STEWARD_FORCE_MANUAL kill-switch can force manual regardless of this value.
	Manual bool
}

func NewInteractiveRunner(cfg *config.Config) *InteractiveRunner {
	return &InteractiveRunner{
		Config: cfg,
	}
}

func (r *InteractiveRunner) RunInteractive(ctx context.Context, feat *feature.Feature, phase, promptText string, opts InteractiveOptions) (*Result, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	phaseCfg, ok := cfg.Phases[phase]
	if !ok {
		return nil, fmt.Errorf("unknown phase: %s", phase)
	}

	agentName := phaseCfg.Agent
	if agentName == "" {
		agentName = cfg.DefaultAgent
	}

	agentCfg, ok := cfg.Agents[agentName]
	if !ok {
		return nil, fmt.Errorf("agent %q not configured", agentName)
	}

	if r.LogDir == "" {
		r.LogDir = filepath.Join(cfg.StewardHome, "logs")
	}
	if err := os.MkdirAll(r.LogDir, 0755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	logFile := filepath.Join(r.LogDir, fmt.Sprintf("%s_%s_%s.log", feat.Name, phase, time.Now().Format("20060102_150405")))
	logFH, err := os.Create(logFile)
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}
	defer logFH.Close()

	PrintPermissionBanner(r.Manual)

	if err := telemetry.RecordPhaseStart(feat, phase, agentName, string(EffectiveMode(r.Manual))); err != nil {
		return nil, fmt.Errorf("record start: %w", err)
	}

	args := r.buildAgentArgs(agentCfg, agentName, phase, promptText)
	cmd := exec.CommandContext(ctx, agentCfg.Cmd, args...)

	workDir := r.ProjectRoot
	if workDir == "" {
		workDir = feat.Dir
	}
	cmd.Dir = workDir

	env := os.Environ()
	for k, v := range agentCfg.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	backend := r.Backend
	if backend == nil {
		backendType := agentCfg.InteractiveBackend
		if backendType == "" {
			backendType = "pty"
		}
		switch backendType {
		case "tmux":
			backend = NewTmuxBackend(r.LogDir)
		default:
			backend = NewPTYBackend(r.LogDir)
		}
	}

	start := time.Now()

	if err := backend.Start(cmd); err != nil {
		return nil, fmt.Errorf("start backend: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	if len(args) == 0 && promptText != "" {
		if stdinProvider, ok := backend.(interface{ Stdin() *os.File }); ok {
			if f := stdinProvider.Stdin(); f != nil {
				fmt.Fprint(f, promptText+"\n")
			}
		}
	}

	reminderDone := make(chan struct{})
	if !opts.NoReminders {
		go r.periodicReminders(backend, phase, feat.DisplayName(), reminderDone)
	}

	timeboxDone := make(chan struct{})
	if opts.Timebox > 0 {
		go r.runTimebox(backend, opts.Timebox, timeboxDone)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	winchDone := make(chan struct{})
	go func() {
		for {
			select {
			case <-sigCh:
				r.handleResize(backend)
			case <-winchDone:
				return
			}
		}
	}()
	defer close(winchDone)
	defer signal.Stop(sigCh)

	r.handleResize(backend)

	waitErr := backend.Wait()

	close(reminderDone)
	if opts.Timebox > 0 {
		close(timeboxDone)
	}

	duration := time.Since(start)

	rawOutput := backend.CapturedOutput()

	if _, err := logFH.WriteString(rawOutput); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write log: %v\n", err)
	}

	backend.Close()

	if !opts.NoExitGuard {
		fmt.Println("\n\nAgent session ended.")
		if !confirmExit() {
			fmt.Println("Wrap-up proceeding...")
		}
	}

	session := CaptureSession(feat, phase, rawOutput, r.ProjectRoot)
	session.SaveArtifacts()

	tokensIn := estimateTokens(promptText)
	tokensOut := estimateTokens(rawOutput)

	exitCode := 0
	errStr := ""
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
		errStr = waitErr.Error()
	}

	if err := telemetry.RecordPhaseEnd(feat, phase, tokensIn, tokensOut, exitCode, 0, errStr); err != nil {
		return nil, fmt.Errorf("record end: %w", err)
	}

	return &Result{
		Stdout:    rawOutput,
		Stderr:    "",
		ExitCode:  exitCode,
		TokensIn:  tokensIn,
		TokensOut: tokensOut,
		Duration:  duration,
		Error:     waitErr,
	}, waitErr
}

func (r *InteractiveRunner) buildAgentArgs(agentCfg *config.AgentConfig, agentName, phase, promptText string) []string {
	// The prompt flag here governs an *interactive* PTY session, which is a
	// different mode from the batch path. agentCfg.PromptFlag holds the batch
	// (non-interactive) flag — e.g. claude's -p/--print, which runs a one-shot
	// and exits immediately, breaking the interactive session. So claude and
	// opencode take the prompt positionally regardless of the configured flag.
	flag := agentCfg.PromptFlag
	switch agentName {
	case "claude", "claude-code", "opencode":
		flag = ""
	case "aider":
		if flag == "" {
			flag = "--message"
		}
	}

	// Permission flags must precede the positional prompt so they parse as options.
	args := PermissionArgs(agentCfg, r.Manual)
	if flag != "" {
		return append(args, flag, promptText)
	}
	return append(args, promptText)
}

func (r *InteractiveRunner) handleResize(backend InteractiveBackend) {
	type resizer interface {
		Resize(rows, cols uint16) error
	}
	if rz, ok := backend.(resizer); ok {
		rows, cols, err := getTerminalSize()
		if err == nil {
			rz.Resize(rows, cols)
		}
	}
}

func getTerminalSize() (uint16, uint16, error) {
	width, height, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return 24, 80, nil
	}
	return uint16(height), uint16(width), nil
}

func (r *InteractiveRunner) periodicReminders(backend InteractiveBackend, phase, featName string, done chan struct{}) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	reminders := []string{
		fmt.Sprintf("\n[steward] You are in the '%s' phase of feature '%s'. Type 'exit' or Ctrl-C to end the session.\n", phase, featName),
		fmt.Sprintf("\n[steward] Still in '%s' phase for '%s'. Focus on the current phase objective.\n", phase, featName),
		fmt.Sprintf("\n[steward] '%s' phase for '%s' — remember to save your work periodically.\n", phase, featName),
	}
	i := 0
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			msg := reminders[i%len(reminders)]
			i++
			if w, ok := backend.(interface{ Stdin() *os.File }); ok {
				fmt.Fprint(w.Stdin(), msg)
			}
		}
	}
}

func (r *InteractiveRunner) runTimebox(backend InteractiveBackend, timeout time.Duration, done chan struct{}) {
	warnAt := timeout * 2 / 3
	warnTimer := time.NewTimer(warnAt)
	defer warnTimer.Stop()
	deadlineTimer := time.NewTimer(timeout)
	defer deadlineTimer.Stop()

	select {
	case <-done:
		return
	case <-warnTimer.C:
		if w, ok := backend.(interface{ Stdin() *os.File }); ok {
			fmt.Fprintf(w.Stdin(), "\n[steward] Warning: %.0f%% of timebox elapsed. Wrap up your current task.\n", 66.0)
		}
	}

	select {
	case <-done:
		return
	case <-deadlineTimer.C:
		if w, ok := backend.(interface{ Stdin() *os.File }); ok {
			fmt.Fprintf(w.Stdin(), "\n[steward] Timebox expired. Ending session.\n")
			fmt.Fprintf(w.Stdin(), "exit\n")
		}
	}
}

func confirmExit() bool {
	drainStdin()
	fmt.Print("Proceed with steward wrap-up? [Y/n]: ")
	var input string
	fmt.Scanln(&input)
	return strings.ToLower(input) != "n" && input != "no"
}

func drainStdin() {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(os.Stdin.Fd()), 0x540B, 0)
}

type InteractiveOptions struct {
	Timebox     time.Duration
	NoReminders bool
	NoExitGuard bool
}
