package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/shahid-io/inode/internal/adapters/db"
	"github.com/shahid-io/inode/internal/core"
	"github.com/shahid-io/inode/internal/model"
)

// maskedContent is what get_note returns in place of a sensitive note's
// plaintext when reveal_sensitive is off. Six bullets matches the CLI
// MaskSensitive default for short values.
const maskedContent = "••••••"

func (h *Handler) handleSearchNotes(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	opts := core.SearchOptions{
		TopK:     int(req.GetFloat("top_k", 0)),
		Category: req.GetString("category", ""),
	}
	if !h.RevealSensitive {
		// Sensitive notes never reach the calling agent — not in
		// the candidate set, not in the formatted result.
		notSensitive := false
		opts.IsSensitive = &notSensitive
	}

	notes, err := h.Container.Search.Retrieve(ctx, query, opts)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if len(notes) == 0 {
		return mcp.NewToolResultText("No relevant notes found."), nil
	}

	return mcp.NewToolResultText(formatSearchResults(query, notes, h.RevealSensitive)), nil
}

func (h *Handler) handleListNotes(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit := int(req.GetFloat("limit", 20))
	filters := db.Filters{
		Category: req.GetString("category", ""),
	}
	if tag := req.GetString("tag", ""); tag != "" {
		filters.Tags = []string{tag}
	}

	notes, err := h.Container.Notes.List(ctx, filters, limit, 0)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if len(notes) == 0 {
		return mcp.NewToolResultText("No notes match the given filters."), nil
	}

	return mcp.NewToolResultText(formatNoteList(notes)), nil
}

func (h *Handler) handleGetNote(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	note, err := h.Container.Notes.Get(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("note %q not found", id)), nil
	}

	return mcp.NewToolResultText(formatNote(note, h.RevealSensitive)), nil
}

// formatSearchResults renders retrieved notes for an MCP agent. The
// agent (Claude, Cursor's model) is itself an LLM — it can read prose
// well, so a lightly-structured block with header + per-note sections
// works better than CSV or JSON.
func formatSearchResults(query string, notes []*model.Note, reveal bool) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d relevant note(s) for: %s\n\n", len(notes), query)
	for i, n := range notes {
		fmt.Fprintf(&sb, "%d. [#%s] %s\n", i+1, shortID(n.ID), n.Summary)
		fmt.Fprintf(&sb, "   category: %s", n.Category)
		if len(n.Tags) > 0 {
			fmt.Fprintf(&sb, " | tags: %s", strings.Join(n.Tags, ", "))
		}
		if n.Distance > 0 {
			fmt.Fprintf(&sb, " | distance: %.3f", n.Distance)
		}
		fmt.Fprintln(&sb)
		fmt.Fprintf(&sb, "   content: %s\n\n", contentFor(n, reveal))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func formatNoteList(notes []*model.Note) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d note(s):\n\n", len(notes))
	for _, n := range notes {
		sensitive := ""
		if n.IsSensitive {
			sensitive = " [sensitive]"
		}
		fmt.Fprintf(&sb, "[#%s] %s — %s", shortID(n.ID), n.Category, n.Summary)
		if len(n.Tags) > 0 {
			fmt.Fprintf(&sb, " (tags: %s)", strings.Join(n.Tags, ", "))
		}
		fmt.Fprintf(&sb, "%s\n", sensitive)
	}
	return strings.TrimRight(sb.String(), "\n")
}

func formatNote(n *model.Note, reveal bool) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "ID: %s\n", n.ID)
	fmt.Fprintf(&sb, "Summary: %s\n", n.Summary)
	fmt.Fprintf(&sb, "Category: %s\n", n.Category)
	if len(n.Tags) > 0 {
		fmt.Fprintf(&sb, "Tags: %s\n", strings.Join(n.Tags, ", "))
	}
	fmt.Fprintf(&sb, "Sensitive: %t\n", n.IsSensitive)
	if !n.CreatedAt.IsZero() {
		fmt.Fprintf(&sb, "Created: %s\n", n.CreatedAt.Format(time.RFC3339))
	}
	fmt.Fprintf(&sb, "\nContent:\n%s\n", contentFor(n, reveal))
	return strings.TrimRight(sb.String(), "\n")
}

func contentFor(n *model.Note, reveal bool) string {
	if n.IsSensitive && !reveal {
		return maskedContent
	}
	if n.ContentPlain != "" {
		return n.ContentPlain
	}
	// Sensitive notes have plaintext only after the service decrypts them.
	// If we ended up here for a sensitive note with no plaintext, it means
	// decryption was skipped — fall back to mask rather than leak metadata.
	if n.IsSensitive {
		return maskedContent
	}
	return ""
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
