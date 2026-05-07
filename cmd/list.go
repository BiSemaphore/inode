package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/shahid-io/inode/internal/adapters/db"
	"github.com/shahid-io/inode/internal/core"
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
		if listSensitive {
			if !prompt("Show sensitive notes?") {
				return nil
			}
		}

		c, err := getContainer()
		if err != nil {
			return err
		}

		filters := db.Filters{
			Category: listCategory,
		}
		if listTag != "" {
			filters.Tags = []string{listTag}
		}
		if listSensitive {
			t := true
			filters.IsSensitive = &t
		}

		notes, err := c.Notes.List(cmd.Context(), filters, listLimit, 0)
		if err != nil {
			return err
		}

		if len(notes) == 0 {
			fmt.Println("No notes found.")
			return nil
		}

		// Mask sensitive content for display.
		notes = core.MaskSensitive(notes)

		fmt.Printf("%-8s  %-40s  %-12s  %-20s  %s\n",
			"ID", "SUMMARY", "CATEGORY", "TAGS", "SAVED")
		fmt.Println(strings.Repeat("─", 100))

		for _, n := range notes {
			tags := strings.Join(n.Tags, ", ")
			if len(tags) > 20 {
				tags = tags[:17] + "..."
			}
			summary := n.Summary
			if len(summary) > 40 {
				summary = summary[:37] + "..."
			}
			sensitive := ""
			if n.IsSensitive {
				sensitive = " ••"
			}
			fmt.Printf("%-8s  %-40s  %-12s  %-20s  %s%s\n",
				n.ID[:8],
				summary,
				n.Category,
				tags,
				ago(n.CreatedAt),
				sensitive,
			)
		}
		return nil
	},
}

func init() {
	listCmd.Flags().StringVar(&listCategory, "category", "", "Filter by category")
	listCmd.Flags().StringVar(&listTag, "tag", "", "Filter by tag")
	listCmd.Flags().BoolVar(&listSensitive, "sensitive", false, "Show sensitive notes (requires confirmation)")
	listCmd.Flags().IntVar(&listLimit, "limit", 20, "Maximum number of notes to return")
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
