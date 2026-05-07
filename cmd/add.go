package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	addCategory  string
	addTags      []string
	addSensitive bool
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
		// TODO(Phase 1): implement note add
		// 1. Get content from args or open $EDITOR
		// 2. TaggerService.Classify() → category, tags, is_sensitive, summary
		// 3. EmbeddingAdapter.Embed(content) → vector
		// 4. Encrypt if sensitive
		// 5. DBAdapter.Save(note)
		fmt.Println("not implemented yet")
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
