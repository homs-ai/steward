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

var analysisCmd = &cobra.Command{
	Use:   "analysis <name>",
	Short: "Convergent thinking phase — structured implementation plan",
	Long:  `Distill brainstorm and research into a concrete, actionable plan with architecture, scope, and implementation blocks.`,
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
		pr.Manual = manualMode
		if err := pr.Analysis(ctx, feat); err != nil {
			return fmt.Errorf("analysis: %w", err)
		}

		fmt.Println("\nAnalysis complete. Review the plan:")
		fmt.Printf("  cat %s/analysis.md\n", feat.Dir)
		fmt.Println("\nRun 'steward implement", name, "' to start coding")
		fmt.Println("Or edit analysis.md and re-run 'steward analysis", name, "' to refine")

		workflow.RequireRating(feat, "analysis")
		return nil
	},
}

func init() {
	analysisCmd.ValidArgsFunction = completeFeatureName
}
