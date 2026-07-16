package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/k/steward/internal/feature"
	"github.com/k/steward/internal/git"
)

var worktreeCmd = &cobra.Command{
	Use:   "worktree",
	Short: "Manage ephemeral git worktrees for parallel feature development",
	Long: `Inspect and control the ephemeral git worktrees steward uses for parallel
development.

  worktree list            Show features and their physical state (In-place / Worktree-live / Dormant)
  worktree add <feature>   Revive a dormant feature into a sibling worktree
  worktree remove <feature> Reclaim a feature's worktree (guards a dirty tree)`,
}

var worktreeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List features and their worktree state",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWorktreeList()
	},
}

var worktreeAddCmd = &cobra.Command{
	Use:   "add <feature>",
	Short: "Revive a dormant feature into a sibling worktree",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWorktreeAdd(args[0])
	},
}

var worktreeRemoveCmd = &cobra.Command{
	Use:   "remove <feature>",
	Short: "Reclaim a feature's worktree (guards a dirty tree)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWorktreeRemove(args[0])
	},
}

func requireRepo() (*git.Runner, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git not found in PATH")
	}
	gr := &git.Runner{Dir: currentProjectRoot}
	if !gr.IsRepo() {
		return nil, fmt.Errorf("not a git repository")
	}
	if gr.IsBare() {
		return nil, fmt.Errorf("bare repositories are not supported")
	}
	return gr, nil
}

func runWorktreeList() error {
	gr, err := requireRepo()
	if err != nil {
		return err
	}

	worktrees, err := gr.WorktreeList()
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}

	// Index live worktrees by canonicalized path for join with feature metadata.
	livePaths := map[string]git.Worktree{}
	for _, wt := range worktrees {
		livePaths[filepath.Clean(wt.Path)] = wt
	}

	features, err := feature.ListFeatures(cfg, currentProject)
	if err != nil {
		return fmt.Errorf("list features: %w", err)
	}

	fmt.Println(strings.Repeat("─", 100))
	fmt.Printf("Project: %s\n", currentProject)
	fmt.Printf("%-22s %-10s %-24s %s\n", "Feature", "State", "Branch", "Worktree")
	fmt.Println(strings.Repeat("─", 100))

	stateColors := map[string]func(a ...interface{}) string{
		"In-place":      color.New(color.FgGreen).SprintFunc(),
		"Worktree-live": color.New(color.FgCyan).SprintFunc(),
		"Dormant":       color.New(color.FgYellow).SprintFunc(),
	}

	for _, f := range features {
		feat, err := feature.Open(cfg, f.Project, f.Name)
		if err != nil {
			continue
		}

		state := "In-place"
		wtDisplay := "-"
		if feat.WorktreePath != "" {
			cleaned := filepath.Clean(feat.WorktreePath)
			if wt, ok := livePaths[cleaned]; ok {
				state = "Worktree-live"
				wtDisplay = feat.WorktreePath
				if wt.Prunable {
					wtDisplay += " (prunable)"
				} else if wt.Locked {
					wtDisplay += " (locked)"
				}
			} else {
				// worktree_path recorded but no live checkout: stale metadata.
				state = "Dormant"
				wtDisplay = feat.WorktreePath + " (stale)"
			}
		}

		branch := feat.BranchName
		if branch == "" {
			branch = "-"
		}

		colorFn := stateColors[state]
		stateDisplay := state
		if colorFn != nil {
			stateDisplay = colorFn(state)
		}

		fmt.Printf("%-22s %-19s %-24s %s\n", f.Name, stateDisplay, branch, wtDisplay)
	}
	fmt.Println(strings.Repeat("─", 100))

	// Surface any prunable worktrees not tied to a steward feature.
	for _, wt := range worktrees {
		if wt.Prunable {
			fmt.Fprintf(os.Stderr, "Note: worktree %s is prunable. Run 'git worktree prune' to clean up.\n", wt.Path)
		}
	}

	return nil
}

func runWorktreeAdd(name string) error {
	gr, err := requireRepo()
	if err != nil {
		return err
	}

	feat, err := feature.Open(cfg, currentProject, name)
	if err != nil {
		return err
	}

	if feat.WorktreePath != "" {
		if _, statErr := os.Stat(feat.WorktreePath); statErr == nil {
			return fmt.Errorf("feature %q already has a live worktree at %s", name, feat.WorktreePath)
		}
	}

	branch := feat.BranchName
	if branch == "" {
		branch = git.SanitizeBranchName(name)
	}

	exists, err := gr.BranchExists(branch)
	if err != nil {
		return fmt.Errorf("check branch exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("branch %q does not exist; cannot revive worktree for it", branch)
	}

	// Guard the one-branch-one-worktree rule.
	if inUse, path, err := branchCheckedOut(gr, branch); err != nil {
		return fmt.Errorf("inspect worktrees: %w", err)
	} else if inUse {
		return fmt.Errorf("branch %q is already checked out at %s", branch, path)
	}

	root, err := repoRoot(gr)
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}
	wtPath := worktreeSiblingPath(root, name)
	if _, err := os.Stat(wtPath); err == nil {
		return fmt.Errorf("worktree path %s already exists; remove it first", wtPath)
	}

	if err := gr.WorktreeAdd(wtPath, branch, false); err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}

	feat.WorktreePath = wtPath
	if err := feat.SaveMetadata(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save worktree metadata: %v\n", err)
	}

	fmt.Printf("Revived feature %q into worktree.\n", name)
	fmt.Printf("  Branch:   %s\n", branch)
	fmt.Printf("  Worktree: %s\n", wtPath)
	fmt.Println()
	fmt.Printf("  cd %s\n", wtPath)
	return nil
}

func runWorktreeRemove(name string) error {
	gr, err := requireRepo()
	if err != nil {
		return err
	}

	feat, err := feature.Open(cfg, currentProject, name)
	if err != nil {
		return err
	}

	if feat.WorktreePath == "" {
		return fmt.Errorf("feature %q has no worktree to remove", name)
	}

	if err := removeWorktree(gr, feat); err != nil {
		return err
	}

	fmt.Printf("Removed worktree for %q. Branch %q and steward state are preserved.\n", name, feat.BranchName)
	return nil
}

func init() {
	worktreeAddCmd.ValidArgsFunction = completeFeatureName
	worktreeRemoveCmd.ValidArgsFunction = completeFeatureName
	worktreeCmd.AddCommand(worktreeListCmd)
	worktreeCmd.AddCommand(worktreeAddCmd)
	worktreeCmd.AddCommand(worktreeRemoveCmd)
	rootCmd.AddCommand(worktreeCmd)
}
