package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/k/steward/internal/feature"
	"github.com/k/steward/internal/telemetry"
)

var metricsAll bool

var metricsCmd = &cobra.Command{
	Use:   "metrics [name]",
	Short: "Show AI metrics dashboard for a feature or project",
	Long:  `Display token usage, cost, iterations, time, and human ratings per phase. Without a feature name, shows all features for the current project. Use --all for all projects.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			project := ""
			if !metricsAll {
				project = currentProject
			}

			features, err := feature.ListFeatures(cfg, project)
			if err != nil {
				return fmt.Errorf("list features: %w", err)
			}
			if len(features) == 0 {
				fmt.Println("No features found.")
				return nil
			}
			for _, f := range features {
				ft, err := telemetry.Load(&f)
				if err != nil {
					fmt.Printf("Error loading telemetry for %s: %v\n", f.DisplayName(), err)
					continue
				}
				fmt.Println()
				telemetry.PrintMetrics(&f, ft, cfg)
			}
			return nil
		}

		name := args[0]
		feat, err := feature.Open(cfg, currentProject, name)
		if err != nil {
			return err
		}
		ft, err := telemetry.Load(feat)
		if err != nil {
			return fmt.Errorf("load telemetry: %w", err)
		}
		telemetry.PrintMetrics(feat, ft, cfg)
		return nil
	},
}

func init() {
	metricsCmd.ValidArgsFunction = completeFeatureName
	metricsCmd.Flags().BoolVarP(&metricsAll, "all", "a", false, "Show metrics across all projects")
}
