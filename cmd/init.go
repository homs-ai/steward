package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/k/steward/internal/config"
	"github.com/k/steward/internal/feature"
)

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize steward or a new feature",
	Long: `Without arguments, interactively sets up steward configuration.
With a feature name, creates a new feature with all required tracking files.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return runProjectInit(cmd)
		}
		return runFeatureInit(args[0])
	},
}

func runProjectInit(cmd *cobra.Command) error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "steward")
	configPath := filepath.Join(configDir, "config.yaml")

	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("Config already exists at %s\n", configPath)
		if !confirm("Overwrite?") {
			fmt.Println("Aborted.")
			return nil
		}
	}

	fmt.Println("\n── Steward Setup ──")
	fmt.Println()

	homeDefault := filepath.Join(os.Getenv("HOME"), ".steward")
	stewardHome := promptWithDefault("Feature artifacts directory", homeDefault)

	codingAgent := promptWithDefault("Coding agent (claude-code, opencode, aider)", "claude-code")
	reviewAgent := promptWithDefault("Review agent (press Enter to use coding agent)", codingAgent)

	agentConfigs := map[string]*config.AgentConfig{
		"claude-code": {
			Cmd:            "claude",
			Phases:         []string{"brainstorm", "research", "analysis", "implement", "test"},
			PromptFlag:     "-p",
			MaxInputTokens: 32000,
			CostPer1KIn:    0.003,
			CostPer1KOut:   0.015,
		},
		"opencode": {
			Cmd:            "opencode",
			Phases:         []string{"brainstorm", "research", "analysis", "implement", "test"},
			PromptFlag:     "--prompt",
			MaxInputTokens: 32000,
			CostPer1KIn:    0.002,
			CostPer1KOut:   0.010,
		},
		"aider": {
			Cmd:            "aider",
			Phases:         []string{"implement"},
			PromptFlag:     "--message",
			MaxInputTokens: 24000,
			CostPer1KIn:    0.002,
			CostPer1KOut:   0.008,
		},
	}

	cfg := &config.Config{
		StewardHome:  stewardHome,
		DefaultAgent: codingAgent,
		Agents:       agentConfigs,
		Phases: map[string]*config.PhaseConfig{
			"init":       {Agent: codingAgent, MaxInputTokens: 8000, HardBlock: false},
			"brainstorm": {Agent: codingAgent, MaxInputTokens: 32000, HardBlock: false},
			"research":   {Agent: codingAgent, MaxInputTokens: 32000, HardBlock: false},
			"analysis":   {Agent: codingAgent, MaxInputTokens: 32000, HardBlock: false},
			"implement":  {Agent: codingAgent, MaxInputTokens: 64000, HardBlock: true},
			"test":       {Agent: codingAgent, MaxInputTokens: 48000, HardBlock: false},
		},
	}

	if reviewAgent != codingAgent {
		if _, ok := cfg.Agents[reviewAgent]; !ok {
			if _, exists := agentConfigs[reviewAgent]; exists {
				cfg.Agents[reviewAgent] = agentConfigs[reviewAgent]
			} else {
				cfg.Agents[reviewAgent] = &config.AgentConfig{
					Cmd:    reviewAgent,
					Phases: []string{},
				}
			}
		}
	}

	if err := os.MkdirAll(stewardHome, 0755); err != nil {
		return fmt.Errorf("create feature home: %w", err)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	fmt.Println("Steward is ready!")
	fmt.Printf("  Config:      %s\n", configPath)
	fmt.Printf("  Artifacts:   %s\n", stewardHome)
	fmt.Printf("  Coding:      %s\n", codingAgent)
	fmt.Printf("  Review:      %s\n", reviewAgent)
	fmt.Println()
	fmt.Println("Next: run 'steward init <feature-name>' to start a feature")
	return nil
}

func runFeatureInit(name string) error {
	f, err := feature.Init(cfg, currentProject, name)
	if err != nil {
		return fmt.Errorf("init feature: %w", err)
	}
	fmt.Printf("Feature %q initialized at %s\n", f.DisplayName(), f.Dir)
	fmt.Printf("Run 'steward brainstorm %s' to explore ideas\n", name)
	return nil
}

func promptWithDefault(label, defaultVal string) string {
	fmt.Printf("%s [%s]: ", label, defaultVal)
	input, err := readLine()
	if err != nil || input == "" {
		return defaultVal
	}
	return input
}
