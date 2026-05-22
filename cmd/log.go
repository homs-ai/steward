package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/k/steward/internal/feature"
	"github.com/k/steward/internal/telemetry"
)

var logCmd = &cobra.Command{
	Use:   "log <name>",
	Short: "Show execution timeline for a feature",
	Long:  `Display a timeline of all phase executions with timestamps and duration.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		feat, err := feature.Open(cfg, currentProject, name)
		if err != nil {
			return err
		}

		ft, err := telemetry.Load(feat)
		if err != nil {
			return fmt.Errorf("load telemetry: %w", err)
		}

		phases := []string{"brainstorm", "research", "analysis", "implement", "test"}
		hasAny := false

		for _, phase := range phases {
			p := ft.Phases[phase]
			if p == nil || p.StartedAt == "" {
				continue
			}
			hasAny = true

			startTime, _ := time.Parse(time.RFC3339, p.StartedAt)
			endTime, _ := time.Parse(time.RFC3339, p.CompletedAt)

			status := "OK"
			if p.Error != "" {
				status = "ERROR"
			}

			fmt.Printf("%s\n", strings.Repeat("─", 60))
			fmt.Printf("Phase: %s\n", phase)
			fmt.Printf("  Agent:    %s\n", p.Agent)
			fmt.Printf("  Start:    %s\n", startTime.Format("Jan 02 15:04:05"))
			fmt.Printf("  End:      %s\n", endTime.Format("Jan 02 15:04:05"))
			fmt.Printf("  Duration: %s\n", telemetry.FormatDuration(p.DurationSec))
			fmt.Printf("  Tokens:   %d in / %d out\n", p.TokensIn, p.TokensOut)
			fmt.Printf("  Cost:     %s\n", telemetry.FormatCost(telemetry.EstimateCost(cfg, p.Agent, p.TokensIn, p.TokensOut)))
			fmt.Printf("  Iter:     %d\n", p.Iterations)
			fmt.Printf("  Status:   %s\n", status)
			if p.HumanRating > 0 {
				fmt.Printf("  Rating:   %d/5\n", p.HumanRating)
			}
			if p.Error != "" {
				fmt.Printf("  Error:    %s\n", p.Error)
			}
		}

		if !hasAny {
			fmt.Printf("No execution data for feature %q yet.\n", feat.DisplayName())
		}
		return nil
	},
}
