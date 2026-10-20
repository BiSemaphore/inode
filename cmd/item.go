package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/BiSemaphore/inode/internal/core"
	"github.com/spf13/cobra"
)

// kind names a stored item by its category, in the singular, for prompts and
// output ("credential #3f1a9c02"). The catch-all category is just an "item".
func kind(category string) string {
	switch category {
	case "", "notes":
		return "item"
	case "credentials", "commands", "snippets", "decisions", "runbooks", "learnings", "references", "contacts":
		return strings.TrimSuffix(category, "s")
	}
	return category
}

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a stored item by ID or unique ID prefix",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

// minDeletePrefix is the shortest ID prefix `inode delete` accepts. inode
// list shows 8 characters; a shorter prefix is fine as long as it is unique.
const minDeletePrefix = 4

var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a stored item by ID or unique ID prefix (at least 4 characters)",
	Args:  cobra.ExactArgs(1),
	RunE:  runDelete,
}

func runShow(cmd *cobra.Command, args []string) error {
	c, err := getContainer()
	if err != nil {
		return err
	}

	it, err := c.Notes.Get(cmd.Context(), args[0])
	if err != nil {
		return err
	}

	content := it.ContentPlain
	if it.IsSensitive && content != "" {
		if !prompt("Reveal sensitive content?") {
			content = "••••••"
		}
	}

	fmt.Printf("ID:        %s\n", it.ID)
	fmt.Printf("Summary:   %s\n", it.Summary)
	fmt.Printf("Category:  %s\n", it.Category)
	fmt.Printf("Tags:      %s\n", strings.Join(it.Tags, ", "))
	fmt.Printf("Sensitive: %v\n", it.IsSensitive)
	fmt.Printf("Created:   %s\n", it.CreatedAt.Format("2006-01-02 15:04"))
	fmt.Printf("Content:\n%s\n", content)
	return nil
}

func runDelete(cmd *cobra.Command, args []string) error {
	ref := strings.TrimSpace(args[0])
	if len(ref) < minDeletePrefix {
		return fmt.Errorf("ID %q is too short: give at least %d characters", ref, minDeletePrefix)
	}

	c, err := getContainer()
	if err != nil {
		return err
	}

	it, err := c.Notes.Resolve(cmd.Context(), ref)
	if err != nil {
		return err
	}

	// Name the item itself, not the user's input. Summaries of sensitive
	// items are scrubbed of secrets (#53), so they are safe to show.
	k := kind(it.Category)
	if !prompt(fmt.Sprintf("Delete %s #%s %q?", k, core.ShortID(it.ID), it.Summary)) {
		fmt.Println("aborted")
		return nil
	}

	if err := c.Notes.Delete(cmd.Context(), it.ID); err != nil {
		return err
	}
	fmt.Printf("deleted %s #%s\n", k, core.ShortID(it.ID))
	return nil
}

// deprecated wraps a RunE for the old `inode note ...` spelling (#145): it
// still works for one release, with a notice on stderr.
func deprecated(newCmd string, run func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		fmt.Fprintf(os.Stderr, "inode %s is deprecated; use inode %s\n", cmd.CommandPath()[len("inode "):], newCmd)
		return run(cmd, args)
	}
}
