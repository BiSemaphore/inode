package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var noteCmd = &cobra.Command{
	Use:   "note",
	Short: "Manage individual notes",
}

var noteGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Fetch a note by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO(Phase 1): implement note get
		fmt.Println("not implemented yet")
		return nil
	},
}

var noteEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit a note in $EDITOR",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO(Phase 1): implement note edit
		fmt.Println("not implemented yet")
		return nil
	},
}

var noteDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a note by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO(Phase 1): implement note delete
		fmt.Println("not implemented yet")
		return nil
	},
}

func init() {
	noteCmd.AddCommand(noteGetCmd)
	noteCmd.AddCommand(noteEditCmd)
	noteCmd.AddCommand(noteDeleteCmd)
}
