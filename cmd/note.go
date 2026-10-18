package cmd

import (
	"fmt"
	"strings"

	"github.com/BiSemaphore/inode/internal/core"
	"github.com/spf13/cobra"
)

var noteCmd = &cobra.Command{
	Use:   "note",
	Short: "Manage individual notes",
}

var noteGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Fetch a note by ID or unique ID prefix",
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

// minDeletePrefix is the shortest ID prefix `note delete` accepts. inode list
// shows 8 characters; a shorter prefix is fine as long as it is unique.
const minDeletePrefix = 4

var noteDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a note by ID or unique ID prefix (at least 4 characters)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ref := strings.TrimSpace(args[0])
		if len(ref) < minDeletePrefix {
			return fmt.Errorf("note ID %q is too short: give at least %d characters", ref, minDeletePrefix)
		}

		c, err := getContainer()
		if err != nil {
			return err
		}

		note, err := c.Notes.Resolve(cmd.Context(), ref)
		if err != nil {
			return err
		}

		// Name the note itself, not the user's input. Summaries of sensitive
		// notes are scrubbed of secrets (#53), so they are safe to show.
		if !prompt(fmt.Sprintf("Delete note #%s %q [%s]?", core.ShortID(note.ID), note.Summary, note.Category)) {
			fmt.Println("aborted")
			return nil
		}

		if err := c.Notes.Delete(cmd.Context(), note.ID); err != nil {
			return err
		}
		fmt.Printf("deleted #%s\n", core.ShortID(note.ID))
		return nil
	},
}

func init() {
	noteCmd.AddCommand(noteGetCmd)
	noteCmd.AddCommand(noteDeleteCmd)
}
