package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/k/steward/internal/config"
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
			project = filepath.Base(cwd)
		}
		currentProject = project

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
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
