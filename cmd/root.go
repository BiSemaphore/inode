package cmd

import (
	"fmt"
	"os"

	"github.com/shahid-io/inode/internal/config"
	"github.com/shahid-io/inode/internal/core"
	"github.com/shahid-io/inode/internal/version"
	"github.com/spf13/cobra"
)

var (
	cfg       *config.Config
	container *core.Container
)

var rootCmd = &cobra.Command{
	Use:   "inode",
	Short: "Personal knowledge base and secret vault — semantic search via vector embeddings and RAG",
	Long: `inode stores notes, secrets, commands, and decisions locally.
Retrieve anything later using natural language via vector similarity search and LLM inference.

Config file: ~/.inode/config.toml
Data:        ~/.inode/notes.db`,
	SilenceUsage: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("inode %s (commit: %s, built: %s)\n", version.Version, version.Commit, version.Date)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(noteCmd)
	rootCmd.AddCommand(configCmd)
}

func initConfig() {
	var err error
	cfg, err = config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// getContainer lazily builds the service container on first use.
// Commands that need adapters call this; config-only commands (version, config show) do not.
func getContainer() (*core.Container, error) {
	if container != nil {
		return container, nil
	}
	var err error
	container, err = core.NewContainer(cfg)
	return container, err
}
