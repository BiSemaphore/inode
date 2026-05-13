package cmd

import (
	"github.com/shahid-io/inode/internal/mcp"
	"github.com/shahid-io/inode/internal/version"
	"github.com/spf13/cobra"
)

// mcpCmd runs inode as a Model Context Protocol server over stdio.
// Intended to be launched by an MCP-aware client (Claude Code, Cursor,
// etc.) — not interactively. The client speaks JSON-RPC over stdio.
//
// Example MCP client config:
//
//	{
//	  "mcpServers": {
//	    "inode": { "command": "inode", "args": ["mcp"] }
//	  }
//	}
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run as an MCP server over stdio (for Claude Code / Cursor)",
	Long: `Run inode as a Model Context Protocol server over stdio.

Exposes three read-only tools to the calling agent:
  search_notes   — vector search; returns the most relevant notes
  list_notes     — paginated listing; metadata only
  get_note       — fetch a single note by ID prefix

Sensitive notes are excluded from search results and masked in get_note
responses by default. To allow the agent to see sensitive content:

    inode config set mcp.reveal_sensitive true

This command is meant to be wired into an MCP client config, not run
interactively.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getContainer()
		if err != nil {
			return err
		}
		h := &mcp.Handler{
			Container:       c,
			RevealSensitive: cfg.MCP.RevealSensitive,
		}
		s := mcp.NewServer(version.Version, h)
		return mcp.Serve(s)
	},
}
