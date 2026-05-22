package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or edit steward configuration",
	Long: `Display the current configuration or open the config file for editing.
Configuration file: ~/.config/steward/config.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := filepath.Join(os.Getenv("HOME"), ".config", "steward", "config.yaml")

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Println("No config file found. Defaults are being used.")
			fmt.Println("Run 'steward agent set <phase> <agent>' to customize.")
			fmt.Println("Config will be created at:", configPath)
			return nil
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("read config: %w", err)
		}

		var parsed interface{}
		if err := yaml.Unmarshal(data, &parsed); err == nil {
			pretty, _ := json.MarshalIndent(parsed, "", "  ")
			fmt.Println(string(pretty))
		} else {
			fmt.Println(string(data))
		}

		fmt.Printf("\nConfig file: %s\n", configPath)
		return nil
	},
}
