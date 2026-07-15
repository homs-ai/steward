package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/k/steward/internal/agent"
	"github.com/k/steward/internal/config"
)

var agentSetCmd = &cobra.Command{
	Use:   "agent set <phase> <agent>",
	Short: "Set the agent for a specific phase",
	Long: `Configure which agent to use for a given phase.
Example: steward agent set implement aider
         steward agent set brainstorm claude-code`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		phase := args[0]
		agentName := args[1]

		validPhases := []string{"init", "brainstorm", "research", "analysis", "implement", "test"}
		valid := false
		for _, p := range validPhases {
			if p == phase {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid phase %q. Valid: %v", phase, validPhases)
		}

		if _, ok := cfg.Agents[agentName]; !ok {
			if !agent.CheckAgent(agentName) {
				fmt.Printf("Warning: agent %q not found in PATH and not configured\n", agentName)
			}
			cfg.Agents[agentName] = &config.AgentConfig{
				Cmd:    agentName,
				Phases: []string{phase},
			}
		}

		cfg.Phases[phase].Agent = agentName

		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Phase %q will now use agent %q\n", phase, agentName)
		fmt.Println("Configuration saved to ~/.config/steward/config.yaml")
		return nil
	},
}

func init() {
	agentSetCmd.ValidArgsFunction = completeAgentSetArgs
}
