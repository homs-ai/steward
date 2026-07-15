package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/k/steward/internal/feature"
	"github.com/k/steward/internal/workflow"
)

var implementCmd = &cobra.Command{
	Use:   "implement <name>",
	Short: "Implement a feature with coding and review agents",
	Long: `Run the implementation phase:
Phase 1: Coding agent builds the feature
Phase 2: Captures diff and generates report`,
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
		if err := pr.Implement(ctx, feat); err != nil {
			return fmt.Errorf("implement: %w", err)
		}

		fmt.Println("\nImplementation complete.")
		fmt.Println("\nNext steps:")
		fmt.Println("  1. Review the diff:", feat.Dir+"/diff.md")
		fmt.Println("  2. Commit changes or amend")
		fmt.Println("  3. Run 'steward test", name, "' for testing")

		workflow.RequireRating(feat, "implement")
		return nil
	},
}

func init() {
	implementCmd.ValidArgsFunction = completeFeatureName
}
