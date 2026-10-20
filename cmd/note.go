package cmd

import "github.com/spf13/cobra"

// noteCmd keeps the pre-#145 spelling `inode note get|delete` working for one
// release. It is hidden from help; use `inode show` and `inode delete`.
var noteCmd = &cobra.Command{
	Use:    "note",
	Short:  "Deprecated: use inode show and inode delete",
	Hidden: true,
}

var noteGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Deprecated: use inode show",
	Args:  cobra.ExactArgs(1),
	RunE:  deprecated("show", runShow),
}

var noteDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Deprecated: use inode delete",
	Args:  cobra.ExactArgs(1),
	RunE:  deprecated("delete", runDelete),
}

func init() {
	noteCmd.AddCommand(noteGetCmd)
	noteCmd.AddCommand(noteDeleteCmd)
}
