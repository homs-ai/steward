package discovery

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Result struct {
	Command  string        `json:"command"`
	Output   string        `json:"output"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
	Timeout  bool          `json:"timeout"`
}

type CommandSet struct {
	ProjectRoot string
	Timeout     time.Duration
}

func NewCommandSet(projectRoot string, timeout time.Duration) *CommandSet {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &CommandSet{
		ProjectRoot: projectRoot,
		Timeout:     timeout,
	}
}

func (cs *CommandSet) Run(ctx context.Context, command string) Result {
	ctx, cancel := context.WithTimeout(ctx, cs.Timeout)
	defer cancel()

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return Result{Command: command, Error: "empty command"}
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = cs.ProjectRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := Result{
		Command:  command,
		Output:   stdout.String(),
		Duration: duration,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Timeout = true
			result.Error = "command timed out"
		} else {
			result.Error = fmt.Sprintf("%v: %s", err, stderr.String())
		}
	}

	return result
}

func (cs *CommandSet) RunAll(ctx context.Context, commands []string) []Result {
	results := make([]Result, 0, len(commands))
	for _, cmd := range commands {
		results = append(results, cs.Run(ctx, cmd))
	}
	return results
}

var StandardCommands = []string{
	"go version",
	"go list -m -json all",
	"go mod graph",
	"node --version",
	"npm ls --all",
	"rustc --version",
	"cargo tree",
	"docker-compose config",
	"docker ps",
	"ss -tlnp",
	"ps aux",
}

func FilterByEcosystem(patterns []string, commands []string) []string {
	ecosystemMap := map[string][]string{
		"go":           {"go version", "go list -m -json all", "go mod graph"},
		"go-workspace": {"go version", "go list -m -json all", "go mod graph"},
		"node":         {"node --version", "npm ls --all"},
		"rust":         {"rustc --version", "cargo tree"},
		"python":       {"python --version", "pip freeze"},
		"java":         {"java -version"},
		"dotnet":       {"dotnet --version", "dotnet list package"},
		"docker":       {"docker-compose config", "docker ps", "docker image inspect"},
		"system":       {"ss -tlnp", "ps aux"},
	}

	seen := make(map[string]bool)
	var filtered []string

	for _, pattern := range patterns {
		if cmds, ok := ecosystemMap[pattern]; ok {
			for _, cmd := range cmds {
				if !seen[cmd] {
					seen[cmd] = true
					filtered = append(filtered, cmd)
				}
			}
		}
	}

	if len(filtered) == 0 {
		return commands
	}
	return filtered
}

func ResultsToText(results []Result) string {
	var sb strings.Builder
	for _, r := range results {
		sb.WriteString(fmt.Sprintf("--- %s ---\n", r.Command))
		if r.Timeout {
			sb.WriteString("[TIMEOUT]\n")
		} else if r.Error != "" {
			sb.WriteString(fmt.Sprintf("[ERROR: %s]\n", r.Error))
		}
		if r.Output != "" {
			sb.WriteString(r.Output)
			if !strings.HasSuffix(r.Output, "\n") {
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
