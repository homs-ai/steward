package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/k/steward/internal/agent"
	"github.com/k/steward/internal/config"
	"github.com/k/steward/internal/feature"
)

var validPhases = []string{"init", "brainstorm", "research", "analysis", "implement", "test"}

func completeFeatureName(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	cfg, err := config.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	project, _ := cmd.Flags().GetString("project")
	features, err := feature.ListFeatures(cfg, project)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var comps []cobra.Completion
	for _, f := range features {
		comps = append(comps, cobra.CompletionWithDesc(f.Name, f.Project))
	}
	return comps, cobra.ShellCompDirectiveNoFileComp
}

func completeAgentSetArgs(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		return completePhases(toComplete)
	case 1:
		return completeAgents(toComplete)
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func completePhases(toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	var comps []cobra.Completion
	for _, p := range validPhases {
		comps = append(comps, cobra.CompletionWithDesc(p, "phase"))
	}
	return comps, cobra.ShellCompDirectiveNoFileComp
}

func completeAgents(toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	cfg, err := config.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	seen := map[string]bool{}
	var comps []cobra.Completion
	for name := range cfg.Agents {
		if !seen[name] {
			comps = append(comps, cobra.CompletionWithDesc(name, "configured"))
			seen[name] = true
		}
	}
	for _, name := range agent.AvailableAgents() {
		if !seen[name] {
			comps = append(comps, cobra.CompletionWithDesc(name, "in PATH"))
			seen[name] = true
		}
	}
	return comps, cobra.ShellCompDirectiveNoFileComp
}

func completeProjectName(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	cfg, err := config.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	entries, err := os.ReadDir(cfg.StewardHome)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var comps []cobra.Completion
	for _, e := range entries {
		if e.IsDir() {
			comps = append(comps, cobra.CompletionWithDesc(e.Name(), "project"))
		}
	}
	return comps, cobra.ShellCompDirectiveNoFileComp
}
