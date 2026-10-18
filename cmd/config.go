package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BiSemaphore/inode/internal/config"
	"github.com/BiSemaphore/inode/internal/core"
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
` + keyHelp(),
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, raw := args[0], args[1]

		key, ok := config.LookupKey(name)
		if !ok {
			return fmt.Errorf("unknown config key %q (see inode config set --help)", name)
		}
		value, err := key.Parse(raw)
		if err != nil {
			return err
		}

		configDir, err := config.ConfigDir()
		if err != nil {
			return err
		}
		if err := config.Set(configDir, name, value); err != nil {
			return fmt.Errorf("write config: %w", err)
		}

		display := raw
		if key.Secret {
			display = redact(raw)
		}
		fmt.Printf("set %s = %s\n", name, display)
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print current configuration (secrets redacted)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dir, err := config.ConfigDir(); err == nil {
			if mode, loose := config.LooseMode(dir); loose {
				path := filepath.Join(dir, config.FileName)
				fmt.Fprintf(os.Stderr, "warning: %s is readable by other users (mode %o); it can hold API keys. Fix: chmod 600 %s\n", path, mode, path)
			}
		}
		// Print what the commands will use: model, base URL and dimension
		// left unset come from the selected backend (#31).
		effective, err := core.ResolveBackends(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
		if effective == nil {
			effective = cfg
		}
		set, values := config.Values(cfg), config.Values(effective)
		for _, k := range config.Keys {
			v := fmt.Sprint(values[k.Name])
			if k.Secret {
				v = redact(v)
			}
			if values[k.Name] != set[k.Name] {
				backend := effective.LLM.Backend
				if strings.HasPrefix(k.Name, "embedding.") {
					backend = effective.Embedding.Backend
				}
				v += fmt.Sprintf("  (default for %s)", backend)
			}
			fmt.Printf("%-21s %s\n", k.Name, v)
		}
		return nil
	},
}

// keyHelp lists the settable keys for `inode config set --help`.
func keyHelp() string {
	var sb strings.Builder
	for _, k := range config.Keys {
		hint := k.Help
		if len(k.Allowed) > 0 {
			hint = strings.Join(k.Allowed, " | ") + ": " + hint
		} else if k.Kind != config.String {
			hint = k.Kind.String() + ": " + hint
		}
		fmt.Fprintf(&sb, "  %-21s %s\n", k.Name, hint)
	}
	return strings.TrimRight(sb.String(), "\n")
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
