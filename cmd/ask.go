package cmd

import (
	"fmt"

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
		// TODO(Phase 1): implement RAG search
		// 1. EmbeddingAdapter.Embed(query) → query vector
		// 2. DBAdapter.SearchSimilar(vec, topK=5, filters)
		// 3. Decrypt sensitive notes in memory
		// 4. LLMAdapter.Answer(query, notes) → response
		// 5. Print response, masking sensitive values unless --reveal
		fmt.Println("not implemented yet")
		return nil
	},
}

func init() {
	askCmd.Flags().BoolVar(&askReveal, "reveal", false, "Unmask sensitive values (requires confirmation)")
}
