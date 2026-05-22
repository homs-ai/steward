package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/k/steward/internal/config"
	"github.com/k/steward/internal/feature"
	"github.com/k/steward/internal/telemetry"
)

type Result struct {
	Stdout    string
	Stderr    string
	ExitCode  int
	TokensIn  int
	TokensOut int
	Duration  time.Duration
	Error     error
}

type Runner struct {
	Config *config.Config
	Stdout bool
}

func NewRunner(cfg *config.Config) *Runner {
	return &Runner{Config: cfg, Stdout: true}
}

func (r *Runner) Run(ctx context.Context, feat *feature.Feature, phase, prompt string, projectRoot string) (*Result, error) {
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

	if err := telemetry.RecordPhaseStart(feat, phase, agentName); err != nil {
		return nil, fmt.Errorf("record start: %w", err)
	}

	args := []string{"run", "--dangerously-skip-permissions", prompt}
	if agentCfg.Cmd == "claude" {
		args = []string{"--dangerously-skip-permissions", prompt}
	}
	cmd := exec.CommandContext(ctx, agentCfg.Cmd, args...)

	workDir := projectRoot
	if workDir == "" {
		workDir = feat.Dir
	}
	cmd.Dir = workDir

	env := os.Environ()
	for k, v := range agentCfg.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err = cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	errStr := ""
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
		errStr = err.Error()
	}

	tokensIn := estimateTokens(prompt)
	tokensOut := estimateTokens(stdout.String())

	if err := telemetry.RecordPhaseEnd(feat, phase, tokensIn, tokensOut, exitCode, 0, errStr); err != nil {
		return nil, fmt.Errorf("record end: %w", err)
	}

	result := &Result{
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
		ExitCode:  exitCode,
		TokensIn:  tokensIn,
		TokensOut: tokensOut,
		Duration:  duration,
		Error:     err,
	}

	if r.Stdout && result.Stdout != "" {
		fmt.Println(result.Stdout)
	}

	return result, nil
}

func estimateTokens(text string) int {
	return len(strings.Fields(text)) + (len(text) / 4)
}

func CheckAgent(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func AvailableAgents() []string {
	candidates := []string{"claude", "opencode", "aider"}
	var available []string
	for _, c := range candidates {
		if CheckAgent(c) {
			available = append(available, c)
		}
	}
	return available
}
