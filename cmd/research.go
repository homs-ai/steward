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

var researchCmd = &cobra.Command{
	Use:   "research <name>",
	Short: "Grounding phase — validate brainstorm ideas",
	Long:  `Research feasibility, existing tools, APIs, and alternatives for a feature.`,
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
		if err := pr.Research(ctx, feat); err != nil {
			return fmt.Errorf("research: %w", err)
		}

		fmt.Println("\nResearch complete. Review findings:")
		fmt.Printf("  cat %s/research.md\n", feat.Dir)
		fmt.Println("\nRun 'steward analysis", name, "' to converge on a solution")
		fmt.Println("Or edit research.md and re-run 'steward research", name, "' to refine")

		workflow.RequireRating(feat, "research")
		return nil
	},
}

func init() {
	researchCmd.ValidArgsFunction = completeFeatureName
}
