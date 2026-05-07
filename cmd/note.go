package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var noteCmd = &cobra.Command{
	Use:   "note",
	Short: "Manage individual notes",
}

var noteGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Fetch a note by ID (prefix match)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getContainer()
		if err != nil {
			return err
		}

		note, err := c.Notes.Get(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		content := note.ContentPlain
		if note.IsSensitive && content != "" {
			if !prompt("Reveal sensitive content?") {
				content = "••••••"
			}
		}

		fmt.Printf("ID:        %s\n", note.ID)
		fmt.Printf("Summary:   %s\n", note.Summary)
		fmt.Printf("Category:  %s\n", note.Category)
		fmt.Printf("Tags:      %s\n", strings.Join(note.Tags, ", "))
		fmt.Printf("Sensitive: %v\n", note.IsSensitive)
		fmt.Printf("Created:   %s\n", note.CreatedAt.Format("2006-01-02 15:04"))
		fmt.Printf("Content:\n%s\n", content)
		return nil
	},
}

var noteDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a note by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !prompt(fmt.Sprintf("Delete note %s?", args[0])) {
			fmt.Println("aborted")
			return nil
		}

		c, err := getContainer()
		if err != nil {
			return err
		}

		if err := c.Notes.Delete(cmd.Context(), args[0]); err != nil {
			return err
		}
		fmt.Printf("deleted %s\n", args[0])
		return nil
	},
}

func init() {
	noteCmd.AddCommand(noteGetCmd)
	noteCmd.AddCommand(noteDeleteCmd)
}
