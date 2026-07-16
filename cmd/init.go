package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/k/steward/internal/config"
	"github.com/k/steward/internal/feature"
	"github.com/k/steward/internal/git"
)

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize steward or a new feature",
	Long: `Without arguments, interactively sets up steward configuration.
With a feature name, creates a new feature with all required tracking files.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return runProjectInit(cmd)
		}
		parallel, _ := cmd.Flags().GetBool("parallel")
		return runFeatureInit(args[0], parallel)
	},
}

func init() {
	initCmd.Flags().Bool("parallel", false, "Create an ephemeral sibling git worktree for parallel development")
	initCmd.Flags().Bool("worktree", false, "Alias for --parallel")
	initCmd.Flags().MarkHidden("worktree")
	// Treat --worktree as an alias: if it is set, --parallel takes effect too.
	initCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if wt, _ := cmd.Flags().GetBool("worktree"); wt {
			cmd.Flags().Set("parallel", "true")
		}
		return nil
	}
}

func runProjectInit(cmd *cobra.Command) error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "steward")
	configPath := filepath.Join(configDir, "config.yaml")

	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("Config already exists at %s\n", configPath)
		if !confirm("Overwrite?") {
			fmt.Println("Aborted.")
			return nil
		}
	}

	fmt.Println("\n── Steward Setup ──")
	fmt.Println()

	homeDefault := filepath.Join(os.Getenv("HOME"), ".steward")
	stewardHome := promptWithDefault("Feature artifacts directory", homeDefault)

	codingAgent := promptWithDefault("Coding agent (claude-code, opencode, aider)", "claude-code")
	reviewAgent := promptWithDefault("Review agent (press Enter to use coding agent)", codingAgent)

	agentConfigs := map[string]*config.AgentConfig{
		"claude-code": {
			Cmd:            "claude",
			Phases:         []string{"brainstorm", "research", "analysis", "implement", "test"},
			PromptFlag:     "-p",
			MaxInputTokens: 32000,
			CostPer1KIn:    0.003,
			CostPer1KOut:   0.015,
		},
		"opencode": {
			Cmd:            "opencode",
			Phases:         []string{"brainstorm", "research", "analysis", "implement", "test"},
			PromptFlag:     "--prompt",
			MaxInputTokens: 32000,
			CostPer1KIn:    0.002,
			CostPer1KOut:   0.010,
		},
		"aider": {
			Cmd:            "aider",
			Phases:         []string{"implement"},
			PromptFlag:     "--message",
			MaxInputTokens: 24000,
			CostPer1KIn:    0.002,
			CostPer1KOut:   0.008,
		},
	}

	cfg := &config.Config{
		StewardHome:  stewardHome,
		DefaultAgent: codingAgent,
		Agents:       agentConfigs,
		Phases: map[string]*config.PhaseConfig{
			"init":       {Agent: codingAgent, MaxInputTokens: 8000, HardBlock: false},
			"brainstorm": {Agent: codingAgent, MaxInputTokens: 32000, HardBlock: false},
			"research":   {Agent: codingAgent, MaxInputTokens: 32000, HardBlock: false},
			"analysis":   {Agent: codingAgent, MaxInputTokens: 32000, HardBlock: false},
			"implement":  {Agent: codingAgent, MaxInputTokens: 64000, HardBlock: true},
			"test":       {Agent: codingAgent, MaxInputTokens: 48000, HardBlock: false},
		},
	}

	if reviewAgent != codingAgent {
		if _, ok := cfg.Agents[reviewAgent]; !ok {
			if _, exists := agentConfigs[reviewAgent]; exists {
				cfg.Agents[reviewAgent] = agentConfigs[reviewAgent]
			} else {
				cfg.Agents[reviewAgent] = &config.AgentConfig{
					Cmd:    reviewAgent,
					Phases: []string{},
				}
			}
		}
	}

	if err := os.MkdirAll(stewardHome, 0755); err != nil {
		return fmt.Errorf("create feature home: %w", err)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	fmt.Println("Steward is ready!")
	fmt.Printf("  Config:      %s\n", configPath)
	fmt.Printf("  Artifacts:   %s\n", stewardHome)
	fmt.Printf("  Coding:      %s\n", codingAgent)
	fmt.Printf("  Review:      %s\n", reviewAgent)
	fmt.Println()
	fmt.Println("Next: run 'steward init <feature-name>' to start a feature")
	return nil
}

type branchAction int

const (
	branchFromCurrent branchAction = iota + 1
	branchFromDefault
	branchSkip
)

func promptBranchChoice(gr *git.Runner, featureName, branchName string) branchAction {
	detached, _ := gr.IsDetachedHead()
	current, _ := gr.CurrentBranch()
	defaultBranch, defaultErr := gr.DefaultBranch()

	if detached {
		fmt.Println("Note: HEAD is detached. Branching from the current commit is not available.")
	}

	fmt.Printf("\nCreate git branch %q for %q?\n", branchName, featureName)

	type option struct {
		action branchAction
		label  string
	}
	var options []option

	if !detached && current != "" {
		options = append(options, option{branchFromCurrent, fmt.Sprintf("From current branch (%s)", current)})
	}

	if defaultErr == nil && defaultBranch != "" {
		if detached || current != defaultBranch {
			options = append(options, option{branchFromDefault, fmt.Sprintf("From default branch (%s)", defaultBranch)})
		}
	}

	options = append(options, option{branchSkip, "Skip git branching"})

	for i, opt := range options {
		fmt.Printf("  %d) %s\n", i+1, opt.label)
	}
	fmt.Printf("Choose [1-%d]: ", len(options))

	input, err := readLine()
	if err != nil {
		return branchSkip
	}

	var choice int
	if _, err := fmt.Sscanf(input, "%d", &choice); err != nil {
		return branchSkip
	}
	if choice < 1 || choice > len(options) {
		return branchSkip
	}

	return options[choice-1].action
}

func runGitBranchSetup(gr *git.Runner, action branchAction, featureName, branchName string) (baseBranch string, err error) {
	exists, err := gr.BranchExists(branchName)
	if err != nil {
		return "", fmt.Errorf("check branch exists: %w", err)
	}
	if exists {
		return "", fmt.Errorf("branch %q already exists. Checkout or delete it manually, then re-run 'steward init'", branchName)
	}

	switch action {
	case branchFromCurrent:
		current, err := gr.CurrentBranch()
		if err != nil {
			return "", fmt.Errorf("get current branch: %w", err)
		}
		if err := gr.CreateAndSwitchBranch(branchName); err != nil {
			return "", fmt.Errorf("create branch: %w", err)
		}
		return current, nil

	case branchFromDefault:
		defaultBranch, err := gr.DefaultBranch()
		if err != nil {
			return "", fmt.Errorf("determine default branch: %w", err)
		}

		dirty, err := gr.IsDirty()
		if err != nil {
			return "", err
		}
		if dirty {
			msg := fmt.Sprintf("steward: auto-stash before init %s", featureName)
			if err := gr.StashPush(msg); err != nil {
				return "", fmt.Errorf("stash failed: %w", err)
			}
			fmt.Printf("Changes stashed as %q. Run 'git stash pop' to restore.\n", msg)
		}

		if err := gr.SwitchToBranch(defaultBranch); err != nil {
			return "", fmt.Errorf("switch to default branch %q: %w", defaultBranch, err)
		}
		if err := gr.CreateAndSwitchBranch(branchName); err != nil {
			return "", fmt.Errorf("create branch: %w", err)
		}
		return defaultBranch, nil

	default:
		return "", nil
	}
}

func runFeatureInit(name string, parallel bool) error {
	if parallel {
		return runParallelFeatureInit(name)
	}

	var branchCreated bool
	var branchName string
	var baseBranch string

	if _, err := exec.LookPath("git"); err == nil {
		gr := &git.Runner{Dir: currentProjectRoot}

		if gr.IsRepo() {
			sanitized := git.SanitizeBranchName(name)
			var action branchAction

			if batchMode {
				if cfg.Git != nil && cfg.Git.AutoBranch == "always" {
					action = branchFromDefault
				} else {
					action = branchSkip
				}
			} else {
				action = promptBranchChoice(gr, name, sanitized)
			}

			if action != branchSkip {
				base, err := runGitBranchSetup(gr, action, name, sanitized)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
				} else {
					branchCreated = true
					branchName = sanitized
					baseBranch = base
				}
			}
		} else if !batchMode {
			fmt.Println("Note: not a git repository. Skipping git branch creation.")
		}
	} else if !batchMode {
		fmt.Println("Note: git not found in PATH. Skipping git branch creation.")
	}

	f, err := feature.Init(cfg, currentProject, name)
	if err != nil {
		return fmt.Errorf("init feature: %w", err)
	}

	if branchCreated {
		f.BranchName = branchName
		f.BaseBranch = baseBranch
		if err := f.SaveMetadata(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save branch metadata: %v\n", err)
		}
	}

	fmt.Printf("Feature %q initialized at %s\n", f.DisplayName(), f.Dir)
	if branchCreated {
		fmt.Printf("  Branch: %s (from %s)\n", branchName, baseBranch)
	}
	fmt.Printf("Run 'steward brainstorm %s' to explore ideas\n", name)

	return nil
}

// runParallelFeatureInit creates an ephemeral sibling worktree for the feature
// so it can be developed alongside whatever occupies the main worktree.
func runParallelFeatureInit(name string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("--parallel requires git, which was not found in PATH")
	}

	gr := &git.Runner{Dir: currentProjectRoot}
	if !gr.IsRepo() {
		return fmt.Errorf("--parallel requires a git repository; the current directory is not one")
	}
	if gr.IsBare() {
		return fmt.Errorf("--parallel is not supported in a bare repository")
	}

	root, err := repoRoot(gr)
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}

	branch := git.SanitizeBranchName(name)

	// Guard the one-branch-one-worktree rule.
	if inUse, path, err := branchCheckedOut(gr, branch); err != nil {
		return fmt.Errorf("inspect worktrees: %w", err)
	} else if inUse {
		return fmt.Errorf("branch %q is already checked out at %s", branch, path)
	}
	if exists, err := gr.BranchExists(branch); err != nil {
		return fmt.Errorf("check branch exists: %w", err)
	} else if exists {
		return fmt.Errorf("branch %q already exists; use 'steward worktree add %s' to revive it", branch, name)
	}

	wtPath := worktreeSiblingPath(root, name)
	if _, err := os.Stat(wtPath); err == nil {
		return fmt.Errorf("worktree path %s already exists; remove it or choose a different feature name", wtPath)
	}

	baseBranch, _ := gr.CurrentBranch()

	if err := gr.WorktreeAdd(wtPath, branch, true); err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}

	f, err := feature.Init(cfg, currentProject, name)
	if err != nil {
		return fmt.Errorf("init feature: %w", err)
	}
	f.BranchName = branch
	f.BaseBranch = baseBranch
	f.WorktreePath = wtPath
	if err := f.SaveMetadata(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save feature metadata: %v\n", err)
	}

	fmt.Printf("Feature %q initialized at %s\n", f.DisplayName(), f.Dir)
	fmt.Printf("  Branch:   %s (from %s)\n", branch, baseBranch)
	fmt.Printf("  Worktree: %s\n", wtPath)
	fmt.Println()
	fmt.Printf("  cd %s\n", wtPath)
	fmt.Printf("Run 'steward brainstorm %s' to explore ideas\n", name)

	return nil
}

func promptWithDefault(label, defaultVal string) string {
	fmt.Printf("%s [%s]: ", label, defaultVal)
	input, err := readLine()
	if err != nil || input == "" {
		return defaultVal
	}
	return input
}
