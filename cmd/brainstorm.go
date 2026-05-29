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

var brainstormInput string

var brainstormCmd = &cobra.Command{
	Use:   "brainstorm <name>",
	Short: "Divergent thinking phase for a feature",
	Long:  `Generate ideas, explore solutions, and identify challenges for a feature.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		feat, err := feature.Open(cfg, currentProject, name)
		if err != nil {
			return err
		}

		if brainstormInput == "" {
			fmt.Print("Describe the feature or problem: ")
			input, err := readLine()
			if err != nil {
				return err
			}
			brainstormInput = input
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		pr := workflow.NewPhaseRunner(cfg)
		pr.ProjectRoot = currentProjectRoot
		pr.Interactive = !batchMode
		if err := pr.Brainstorm(ctx, feat, brainstormInput); err != nil {
			return fmt.Errorf("brainstorm: %w", err)
		}

		fmt.Println("\nBrainstorm complete. Review ideas:")
		fmt.Printf("  cat %s/brainstorm.md\n", feat.Dir)
		fmt.Println("\nRun 'steward research", name, "' to validate these ideas")
		fmt.Println("Or edit brainstorm.md and re-run 'steward brainstorm", name, "' with new input")

		workflow.RequireRating(feat, "brainstorm")
		return nil
	},
}

func init() {
	brainstormCmd.Flags().StringVarP(&brainstormInput, "input", "i", "", "Feature description (prompts interactively if empty)")
}
