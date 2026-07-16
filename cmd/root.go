package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/k/steward/internal/config"
	"github.com/k/steward/internal/git"
)

var (
	cfg                *config.Config
	cfgLoaded          bool
	currentProject     string
	currentProjectRoot string
	batchMode          bool
)

var rootCmd = &cobra.Command{
	Use:   "steward",
	Short: "Orchestrate software features with AI coding agents",
	Long: `steward is a CLI tool that puts you in charge as the lead engineer.
It guides features through a structured lifecycle:
init → brainstorm → research → analysis → implement → test

You steer, agents execute. Every phase is tracked and auditable.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			return nil
		}
		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		cfgLoaded = true

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		currentProjectRoot = cwd

		project, err := cmd.Flags().GetString("project")
		if err != nil {
			return err
		}
		if project == "" {
			project, _ = resolveProject(cwd)
		}
		currentProject = project

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// resolveProject derives the steward project name and stable identity key for a
// working directory. When cwd is inside a non-bare git repository, the identity
// key is the canonicalized common git dir (invariant across the main worktree,
// secondary worktrees, and subdirectories) and the name is the repo-root
// basename. This is what collapses ephemeral worktrees onto the same steward
// project. Falls back to the cwd basename when git is absent or the repo is bare.
func resolveProject(cwd string) (name, key string) {
	gr := &git.Runner{Dir: cwd}
	if gr.IsRepo() && !gr.IsBare() {
		common, err := gr.CommonDir()
		if err == nil {
			key = common
			name = filepath.Base(filepath.Dir(common))
			return name, key
		}
	}
	return filepath.Base(cwd), cwd
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP("project", "p", "", "Project name (default: basename of current directory)")
	rootCmd.PersistentFlags().BoolVarP(&batchMode, "batch", "b", false, "Run in batch mode (non-interactive)")
	rootCmd.PersistentFlags().Lookup("project").Annotations = nil
	rootCmd.RegisterFlagCompletionFunc("project", completeProjectName)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(brainstormCmd)
	rootCmd.AddCommand(researchCmd)
	rootCmd.AddCommand(analysisCmd)
	rootCmd.AddCommand(implementCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(metricsCmd)
	rootCmd.AddCommand(agentsCmd)
	rootCmd.AddCommand(agentSetCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(configCmd)
}
