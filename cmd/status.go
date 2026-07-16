package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/k/steward/internal/feature"
)

var statusAll bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show overview of features",
	Long:  `Display a table of all features for the current project, their current phase, and last activity. Use --all to show features across all projects.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		project := ""
		if !statusAll {
			project = currentProject
			warnMovedProject()
		}

		features, err := feature.ListFeatures(cfg, project)
		if err != nil {
			return fmt.Errorf("list features: %w", err)
		}

		if len(features) == 0 {
			scope := currentProject
			if statusAll {
				scope = "any project"
			}
			fmt.Printf("No features found for %s. Run 'steward init <name>' to create one.\n", scope)
			return nil
		}

		phaseColors := map[string]func(format string, a ...interface{}) string{
			"brainstorm": color.New(color.FgCyan).SprintfFunc(),
			"research":   color.New(color.FgBlue).SprintfFunc(),
			"analysis":   color.New(color.FgYellow).SprintfFunc(),
			"implement":  color.New(color.FgMagenta).SprintfFunc(),
			"test":       color.New(color.FgHiBlue).SprintfFunc(),
			"complete":   color.New(color.FgGreen).SprintfFunc(),
		}

		fmt.Println(strings.Repeat("─", 90))
		if statusAll {
			fmt.Printf("%-30s %-20s %-20s\n", "Feature", "Phase", "Last Activity")
		} else {
			fmt.Printf("Project: %s\n", currentProject)
			fmt.Printf("%-25s %-20s %-20s\n", "Feature", "Phase", "Last Activity")
		}
		fmt.Println(strings.Repeat("─", 90))

		for _, f := range features {
			phase := f.FindPhase()
			colorFn := phaseColors[phase]
			if colorFn == nil {
				colorFn = fmt.Sprintf
			}
			phaseDisplay := colorFn(phase)
			lastAct := f.LastActivity().Format("Jan 02 15:04")

			name := f.DisplayName()
			if !statusAll {
				name = f.Name
			}
			fmt.Printf("%-30s %-20s %-20s\n", name, phaseDisplay, lastAct)
		}
		fmt.Println(strings.Repeat("─", 90))
		return nil
	},
}

// warnMovedProject warns when the git-derived project name differs from an
// on-disk project folder that matches the current directory basename. This
// surfaces the "moved project" case (e.g. running from a subdirectory or after a
// repo-dir rename) where features may live under a different project folder.
func warnMovedProject() {
	base := filepath.Base(currentProjectRoot)
	if base == "" || base == currentProject {
		return
	}
	// Only warn if an on-disk project folder for the basename actually exists,
	// i.e. steward state was created under the old (basename) identity.
	oldDir := filepath.Join(cfg.StewardHome, base)
	if info, err := os.Stat(oldDir); err == nil && info.IsDir() {
		fmt.Fprintf(os.Stderr,
			"Note: git-derived project %q differs from on-disk project folder %q.\n"+
				"      Features created before repo-identity resolution may live under %q.\n"+
				"      Use --project %s to view them.\n\n",
			currentProject, base, base, base)
	}
}

func init() {
	statusCmd.Flags().BoolVarP(&statusAll, "all", "a", false, "Show features across all projects")
}
