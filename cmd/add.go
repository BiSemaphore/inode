package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/shahid-io/inode/internal/core"
	"github.com/spf13/cobra"
)

var (
	addCategory    string
	addTags        []string
	addSensitive   bool
	addNoSensitive bool
)

var addCmd = &cobra.Command{
	Use:   "add [content]",
	Short: "Save a note, secret, command, or decision",
	Long: `Save content to inode. If no content is provided, opens $EDITOR.

LLM auto-detects category and tags unless specified.
Notes are flagged sensitive by default (configurable).

Examples:
  inode add "My GitHub PAT is ghp_xxxx"
  inode add "docker system prune -a" --category commands --tags docker,cleanup
  inode add --sensitive "AWS_SECRET=xxxx"
  inode add --no-sensitive "Just a reminder"
  inode add   # opens $EDITOR`,
	RunE: func(cmd *cobra.Command, args []string) error {
		content := strings.Join(args, " ")

		if content == "" {
			var err error
			content, err = openEditor()
			if err != nil {
				return fmt.Errorf("open editor: %w", err)
			}
		}

		content = strings.TrimSpace(content)
		if content == "" {
			return fmt.Errorf("no content provided")
		}

		c, err := getContainer()
		if err != nil {
			return err
		}

		opts := core.AddOptions{
			DefaultSensitive: cfg.Defaults.Sensitive,
		}
		opts.Category = addCategory
		opts.Tags = addTags

		switch {
		case addSensitive:
			t := true
			opts.IsSensitive = &t
		case addNoSensitive:
			f := false
			opts.IsSensitive = &f
		}

		note, err := c.Notes.Add(cmd.Context(), content, opts)
		if err != nil {
			return err
		}

		fmt.Printf("  category=%s  tags=[%s]  sensitive=%v\n",
			note.Category, strings.Join(note.Tags, ", "), note.IsSensitive)
		fmt.Printf("  saved  id=%s\n", note.ID[:8])
		return nil
	},
}

func init() {
	addCmd.Flags().StringVar(&addCategory, "category", "", "Override auto-detected category")
	addCmd.Flags().StringSliceVar(&addTags, "tags", nil, "Override auto-detected tags (comma-separated)")
	addCmd.Flags().BoolVar(&addSensitive, "sensitive", false, "Force mark as sensitive")
	addCmd.Flags().BoolVar(&addNoSensitive, "no-sensitive", false, "Force mark as not sensitive")
	addCmd.MarkFlagsMutuallyExclusive("sensitive", "no-sensitive")
}

func openEditor() (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	tmp, err := os.CreateTemp("", "inode-*.txt")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	tmp.Close()

	c := exec.Command(editor, tmp.Name())
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return "", err
	}
	data, err := os.ReadFile(tmp.Name())
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// prompt reads a y/N confirmation from stdin.
func prompt(question string) bool {
	fmt.Printf("%s [y/N]: ", question)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.EqualFold(strings.TrimSpace(scanner.Text()), "y")
	}
	return false
}
