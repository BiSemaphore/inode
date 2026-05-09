package cmd

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/shahid-io/inode/internal/cli"
	"github.com/shahid-io/inode/internal/core"
	"github.com/shahid-io/inode/internal/model"
	"github.com/spf13/cobra"
)

var getReveal bool

// Output palette. fatih/color auto-disables when output isn't a TTY or
// NO_COLOR is set, so piped/CI runs stay plain.
var (
	colorAnswer    = color.New(color.Bold).SprintFunc()
	colorRule      = color.New(color.Faint).SprintFunc()
	colorNoteID    = color.New(color.FgCyan).SprintFunc()
	colorSummary   = color.New(color.Reset).SprintFunc()
	colorCategory  = color.New(color.FgBlue).SprintFunc()
	colorTags      = color.New(color.FgGreen).SprintFunc()
	colorSensitive = color.New(color.FgRed, color.Bold).SprintFunc()
	colorMiss      = color.New(color.Faint, color.Italic).SprintFunc()
)

var getCmd = &cobra.Command{
	Use:     "get <query>",
	Aliases: []string{"ask", "find", "search"},
	Short:   "Retrieve notes by natural-language query",
	Long: `Embed the query, run vector similarity search, and answer via LLM inference.
Retrieved notes are used as context — only your data is referenced.

Sensitive values are masked by default. Use --reveal to unmask.

Examples:
  inode get "github personal access token"
  inode get "docker cleanup command"
  inode get "stripe test key" --reveal

Aliases: ask, find, search.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")

		c, err := getContainer()
		if err != nil {
			return err
		}

		sp := cli.NewSpinner("preparing")
		opts := core.SearchOptions{
			OnStep: sp.Update,
		}

		result, err := c.Search.Search(cmd.Context(), query, opts)
		sp.Stop()
		if err != nil {
			return err
		}

		// The answer is always printed. The LLM owns the "no relevant
		// notes found" case so the message reads as natural language.
		if len(result.Notes) == 0 {
			fmt.Println(colorMiss(result.Answer))
			return nil
		}
		fmt.Println(colorAnswer(result.Answer))

		notes := result.Notes
		if !getReveal {
			notes = core.MaskSensitive(notes)
		} else if hasSensitive(result.Notes) {
			if !prompt("Reveal sensitive values?") {
				notes = core.MaskSensitive(notes)
			}
		}

		fmt.Println()
		fmt.Printf("%s\n", colorRule(fmt.Sprintf("── Sources (%d note(s)) ──", len(notes))))
		for _, n := range notes {
			tagsCol := colorTags(strings.Join(n.Tags, ", "))
			sensitive := ""
			if n.IsSensitive {
				sensitive = "  " + colorSensitive("[sensitive]")
			}
			fmt.Printf("  %s  %s  [%s] [%s]%s\n",
				colorNoteID("#"+n.ID[:8]),
				colorSummary(n.Summary),
				colorCategory(n.Category),
				tagsCol,
				sensitive,
			)
		}
		return nil
	},
}

func init() {
	getCmd.Flags().BoolVar(&getReveal, "reveal", false, "Unmask sensitive values (requires confirmation)")
}

func hasSensitive(notes []*model.Note) bool {
	for _, n := range notes {
		if n.IsSensitive {
			return true
		}
	}
	return false
}
