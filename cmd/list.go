package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	listCategory  string
	listTag       string
	listSensitive bool
	listLimit     int
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved notes",
	Long: `List notes with optional filters. Sensitive values are masked by default.

Examples:
  inode list
  inode list --category credentials
  inode list --tag github
  inode list --sensitive`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO(Phase 1): implement note listing
		// 1. Build Filters from flags
		// 2. DBAdapter.List(filters, limit, offset)
		// 3. Render table (Lipgloss), mask sensitive values
		// 4. If --sensitive, prompt for confirmation first
		fmt.Println("not implemented yet")
		return nil
	},
}

func init() {
	listCmd.Flags().StringVar(&listCategory, "category", "", "Filter by category")
	listCmd.Flags().StringVar(&listTag, "tag", "", "Filter by tag")
	listCmd.Flags().BoolVar(&listSensitive, "sensitive", false, "Show sensitive notes (requires confirmation)")
	listCmd.Flags().IntVar(&listLimit, "limit", 20, "Maximum number of notes to return")
}
