package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BiSemaphore/inode/internal/config"
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
  llm.backend          claude-api | openai | ollama
  llm.model            e.g. claude-sonnet-4-6, gpt-4o-mini
  llm.api_key          Anthropic or OpenAI API key
  embedding.backend    voyage | local
  embedding.api_key    Voyage AI API key
  defaults.sensitive   true | false`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]

		allowed := map[string]bool{
			"llm.backend": true, "llm.model": true, "llm.api_key": true, "llm.base_url": true,
			"embedding.backend": true, "embedding.model": true, "embedding.api_key": true,
			"embedding.base_url": true, "embedding.dimension": true,
			"db.path": true, "defaults.sensitive": true, "log.level": true,
		}
		if !allowed[key] {
			return fmt.Errorf("unknown config key %q", key)
		}

		configDir, err := config.ConfigDir()
		if err != nil {
			return err
		}
		if err := config.Set(configDir, key, value); err != nil {
			return fmt.Errorf("write config: %w", err)
		}

		displayValue := value
		if strings.HasSuffix(key, "api_key") {
			displayValue = redact(value)
		}
		fmt.Printf("set %s = %s\n", key, displayValue)
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print current configuration (API keys redacted)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dir, err := config.ConfigDir(); err == nil {
			if mode, loose := config.LooseMode(dir); loose {
				path := filepath.Join(dir, config.FileName)
				fmt.Fprintf(os.Stderr, "warning: %s is readable by other users (mode %o); it can hold API keys. Fix: chmod 600 %s\n", path, mode, path)
			}
		}
		fmt.Printf("LLM backend:         %s\n", cfg.LLM.Backend)
		fmt.Printf("LLM model:           %s\n", cfg.LLM.Model)
		fmt.Printf("LLM base URL:        %s\n", cfg.LLM.BaseURL)
		fmt.Printf("LLM API key:         %s\n", redact(cfg.LLM.APIKey))
		fmt.Printf("Embedding backend:   %s\n", cfg.Embedding.Backend)
		fmt.Printf("Embedding model:     %s\n", cfg.Embedding.Model)
		fmt.Printf("Embedding base URL:  %s\n", cfg.Embedding.BaseURL)
		fmt.Printf("Embedding API key:   %s\n", redact(cfg.Embedding.APIKey))
		fmt.Printf("Embedding dimension: %d\n", cfg.Embedding.Dimension)
		fmt.Printf("DB path:             %s\n", cfg.DB.Path)
		fmt.Printf("Default sensitive:   %v\n", cfg.Defaults.Sensitive)
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
