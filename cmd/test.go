package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/k/steward/internal/feature"
	"github.com/k/steward/internal/git"
	"github.com/k/steward/internal/workflow"
)

var testCmd = &cobra.Command{
	Use:   "test <name>",
	Short: "Generate test scope and execute tests for a feature",
	Long: `Two-phase testing process:
Phase 1: Analyzes requirements, review, and report to generate test scope
Phase 2: Executes tests and generates test report`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		feat, err := feature.Open(cfg, currentProject, name)
		if err != nil {
			return err
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		pr := workflow.NewPhaseRunner(cfg)
		pr.ProjectRoot = currentProjectRoot
		pr.Interactive = !batchMode
		if err := pr.Test(ctx, feat); err != nil {
			return fmt.Errorf("test: %w", err)
		}

		fmt.Println("\nTest cycle complete.")
		fmt.Println("Review the test report:")
		fmt.Printf("  cat %s/test_report.md\n", feat.Dir)

		workflow.RequireRating(feat, "test")

		maybeTeardownWorktree(feat)
		return nil
	},
}

// maybeTeardownWorktree offers to remove a feature's ephemeral worktree once the
// feature reaches its terminal phase. It never touches the branch or steward
// state — only the transient checkout. Guards a dirty tree (never force-removes).
func maybeTeardownWorktree(feat *feature.Feature) {
	if feat.WorktreePath == "" {
		return
	}
	if _, err := os.Stat(feat.WorktreePath); err != nil {
		// Worktree path recorded but gone; leave metadata for `worktree list` to flag.
		return
	}
	if batchMode {
		fmt.Printf("\nNote: feature worktree %s still exists (run 'steward worktree remove %s' to reclaim).\n", feat.WorktreePath, feat.Name)
		return
	}

	if !confirm(fmt.Sprintf("\nFeature complete. Remove worktree %s?", feat.WorktreePath)) {
		fmt.Printf("Keeping worktree %s. It will linger until you run 'steward worktree remove %s'.\n", feat.WorktreePath, feat.Name)
		return
	}

	gr := &git.Runner{Dir: currentProjectRoot}
	if err := removeWorktree(gr, feat); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		fmt.Printf("Worktree %s left in place.\n", feat.WorktreePath)
		return
	}
	fmt.Printf("Removed worktree. Branch %q and steward state are preserved.\n", feat.BranchName)
}

func init() {
	testCmd.ValidArgsFunction = completeFeatureName
}
