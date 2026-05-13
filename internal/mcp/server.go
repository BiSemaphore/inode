// Package mcp exposes the inode knowledge base over the Model Context
// Protocol so MCP-aware AI clients (Claude Code, Cursor, etc.) can read
// notes via stdio.
//
// v1 is read-only by design: search_notes, list_notes, get_note. Writing
// notes from an agent would let it fill — or wipe — the user's KB without
// the user's intent showing up anywhere reviewable, so it stays opt-in
// for a future iteration.
//
// Sensitive notes are never revealed to the agent unless mcp.reveal_sensitive
// is explicitly set in config. The default (false) excludes them from
// search candidates entirely (so the LLM cannot quote secrets in answers)
// and masks their content in get_note responses.
package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/shahid-io/inode/internal/core"
)

// Handler owns the inode services the MCP tools delegate to.
type Handler struct {
	Container       *core.Container
	RevealSensitive bool
}

// NewServer builds an MCP server with the inode tool surface registered.
// Caller is responsible for actually serving it (e.g. server.ServeStdio).
func NewServer(version string, h *Handler) *server.MCPServer {
	s := server.NewMCPServer(
		"inode",
		version,
		server.WithToolCapabilities(true),
	)

	s.AddTool(
		mcp.NewTool("search_notes",
			mcp.WithDescription(
				"Search the user's inode knowledge base by natural-language query. "+
					"Returns an LLM-generated answer grounded in the most relevant notes, "+
					"with the source notes listed below. Use this for questions like "+
					"'what's the docker cleanup command' or 'how did we configure auth in service X'.",
			),
			mcp.WithString("query", mcp.Required(),
				mcp.Description("Natural-language search query")),
			mcp.WithNumber("top_k",
				mcp.Description("Max number of source notes to consider (default 5)")),
			mcp.WithString("category",
				mcp.Description("Filter to one category: credentials, commands, snippets, decisions, runbooks, learnings, references, contacts, notes")),
		),
		h.handleSearchNotes,
	)

	s.AddTool(
		mcp.NewTool("list_notes",
			mcp.WithDescription(
				"List notes from the user's inode knowledge base, optionally filtered "+
					"by category or tag. Returns metadata only (id, summary, category, "+
					"tags, sensitivity) — not note content. Use this to browse what's "+
					"stored before deciding what to look up in detail.",
			),
			mcp.WithString("category",
				mcp.Description("Filter by category (see search_notes for the list)")),
			mcp.WithString("tag",
				mcp.Description("Filter by a single tag")),
			mcp.WithNumber("limit",
				mcp.Description("Max notes to return (default 20)")),
		),
		h.handleListNotes,
	)

	s.AddTool(
		mcp.NewTool("get_note",
			mcp.WithDescription(
				"Fetch a single note by ID. The ID may be the full UUID or a short "+
					"prefix (8+ characters is typical and unambiguous). Returns the "+
					"note's content along with its metadata. Sensitive notes are "+
					"returned with masked content unless mcp.reveal_sensitive is set.",
			),
			mcp.WithString("id", mcp.Required(),
				mcp.Description("Note ID (full UUID or short prefix)")),
		),
		h.handleGetNote,
	)

	return s
}

// Serve runs the server over stdio. Blocks until the client disconnects.
func Serve(s *server.MCPServer) error {
	return server.ServeStdio(s)
}
