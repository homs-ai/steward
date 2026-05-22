package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/k/steward/internal/agent"
	"github.com/k/steward/internal/telemetry"
)

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Show cross-feature agent health comparison",
	Long: `Display average tokens, time, cost, fail rate, and ratings per agent across all features.
Use this data to decide which agents perform best for each phase.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		summaries, err := telemetry.AggregateAgents(cfg)
		if err != nil {
			return fmt.Errorf("aggregate agents: %w", err)
		}

		if len(summaries) == 0 {
			fmt.Println("No agent telemetry data yet. Complete a feature first.")
			fmt.Println("\nAvailable agents on this system:")
			for _, a := range agent.AvailableAgents() {
				fmt.Printf("  - %s\n", a)
			}
			return nil
		}

		fmt.Println(strings.Repeat("─", 110))
		fmt.Printf("%-15s %-8s %-12s %-12s %-10s %-8s %-10s %-8s %s\n",
			"Agent", "Feats", "Avg In", "Avg Out", "Avg Time", "Iter", "Fail Rate", "Cost", "Rating")
		fmt.Println(strings.Repeat("─", 110))

		for _, s := range summaries {
			ratingStr := "-"
			if s.AvgRating > 0 {
				ratingStr = fmt.Sprintf("%.1f/5", s.AvgRating)
			}
			fmt.Printf("%-15s %-8d %-12s %-12s %-10s %-8.1f %-8.1f%% %-8s %s\n",
				s.Name, s.Features,
				telemetry.FormatTokens(int(s.AvgTokensIn)),
				telemetry.FormatTokens(int(s.AvgTokensOut)),
				telemetry.FormatDuration(int(s.AvgTimeSec)),
				s.AvgIter, s.FailRate,
				telemetry.FormatCost(s.AvgCost),
				ratingStr)
		}
		fmt.Println(strings.Repeat("─", 110))

		fmt.Println("\nAvailable agents on this system:")
		for _, a := range agent.AvailableAgents() {
			fmt.Printf("  - %s\n", a)
		}
		return nil
	},
}
