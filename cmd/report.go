package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/k/steward/internal/feature"
)

var reportCmd = &cobra.Command{
	Use:   "report <name>",
	Short: "Show full build report for a feature",
	Long:  `Display the complete build report including requirements, analysis, and test results.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		feat, err := feature.Open(cfg, currentProject, name)
		if err != nil {
			return err
		}

		files := []string{"req.md", "brainstorm.md", "research.md", "analysis.md", "report.md", "test_report.md"}
		for _, f := range files {
			content, err := feat.ReadFile(f)
			if err != nil {
				continue
			}
			if content == "" {
				continue
			}
			fmt.Printf("\n%s %s %s\n", "─", f, strings.Repeat("─", 60-len(f)))
			fmt.Println(content)
		}
		return nil
	},
}
