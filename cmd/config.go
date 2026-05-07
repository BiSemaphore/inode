package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage inode configuration",
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Long: `Set a configuration key-value pair in ~/.inode/config.toml.

Keys:
  llm.backend          claude-api | ollama
  llm.model            e.g. claude-sonnet-4-6
  llm.api_key          Anthropic API key
  embedding.backend    voyage | local
  embedding.api_key    Voyage AI API key
  defaults.sensitive   true | false`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO(Phase 1): implement config set
		// Write key=value to ~/.inode/config.toml
		// Encrypt API keys before writing
		fmt.Println("not implemented yet")
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print current configuration (API keys redacted)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO(Phase 1): implement config show
		// Print all config values; redact api_key fields
		fmt.Printf("LLM backend:        %s\n", cfg.LLM.Backend)
		fmt.Printf("LLM model:          %s\n", cfg.LLM.Model)
		fmt.Printf("LLM API key:        %s\n", redact(cfg.LLM.APIKey))
		fmt.Printf("Embedding backend:  %s\n", cfg.Embedding.Backend)
		fmt.Printf("Embedding model:    %s\n", cfg.Embedding.Model)
		fmt.Printf("Embedding API key:  %s\n", redact(cfg.Embedding.APIKey))
		fmt.Printf("DB path:            %s\n", cfg.DB.Path)
		fmt.Printf("Default sensitive:  %v\n", cfg.Defaults.Sensitive)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configShowCmd)
}

func redact(s string) string {
	if s == "" {
		return "(not set)"
	}
	if len(s) <= 8 {
		return "********"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
