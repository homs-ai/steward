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
		if err := pr.Test(ctx, feat); err != nil {
			return fmt.Errorf("test: %w", err)
		}

		fmt.Println("\nTest cycle complete.")
		fmt.Println("Review the test report:")
		fmt.Printf("  cat %s/test_report.md\n", feat.Dir)

		workflow.RequireRating(feat, "test")
		return nil
	},
}
