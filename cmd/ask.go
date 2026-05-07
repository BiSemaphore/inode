package cmd

import (
	"fmt"
	"strings"

	"github.com/shahid-io/inode/internal/core"
	"github.com/shahid-io/inode/internal/model"
	"github.com/spf13/cobra"
)

var askReveal bool

var askCmd = &cobra.Command{
	Use:   "ask <query>",
	Short: "Search notes using natural language",
	Long: `Embed the query, run vector similarity search, and answer via LLM inference.
Retrieved notes are used as context — only your data is referenced.

Sensitive values are masked by default. Use --reveal to unmask.

Examples:
  inode ask "github personal access token"
  inode ask "docker cleanup command"
  inode ask "stripe test key" --reveal`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")

		c, err := getContainer()
		if err != nil {
			return err
		}

		result, err := c.Search.Search(cmd.Context(), query, core.SearchOptions{})
		if err != nil {
			return err
		}

		if len(result.Notes) == 0 {
			fmt.Println("No relevant notes found.")
			return nil
		}

		notes := result.Notes
		if !askReveal {
			notes = core.MaskSensitive(notes)
		} else if hasSensitive(result.Notes) {
			if !prompt("Reveal sensitive values?") {
				notes = core.MaskSensitive(notes)
			}
		}

		fmt.Println(result.Answer)
		fmt.Println()
		fmt.Printf("── Sources (%d note(s)) ──\n", len(notes))
		for _, n := range notes {
			tags := strings.Join(n.Tags, ", ")
			sensitive := ""
			if n.IsSensitive {
				sensitive = "  [sensitive]"
			}
			fmt.Printf("  #%s  %s  [%s] [%s]%s\n",
				n.ID[:8], n.Summary, n.Category, tags, sensitive)
		}
		return nil
	},
}

func init() {
	askCmd.Flags().BoolVar(&askReveal, "reveal", false, "Unmask sensitive values (requires confirmation)")
}

func hasSensitive(notes []*model.Note) bool {
	for _, n := range notes {
		if n.IsSensitive {
			return true
		}
	}
	return false
}
