package model

import (
	"strings"
	"time"
)

type Note struct {
	ID           string
	UserID       string    // empty in Phase 1 (local, single-user)
	ContentEnc   []byte    // iv[12] + gcm_sealed[N]; nil if not sensitive
	ContentPlain string    // empty if sensitive
	Summary      string    // LLM-generated one-liner
	Category     string    // see Categories below — strict, validated via IsValidCategory
	Tags         []string
	IsSensitive  bool
	Embedding    []float32 // voyage-3: 1024 dims
	CreatedAt    time.Time
	UpdatedAt    time.Time

	// Distance is the L2 distance from the query vector when this note was
	// returned by a similarity search. 0 if not from a search. Not persisted.
	Distance float32
}

// Categories is the strict, predefined set of note categories. The LLM
// must pick from this list — values outside it are rejected by the
// classifier and fall back to FallbackCategory.
//
// Adding a category here is a deliberate act: existing notes don't get
// reclassified, and a category that overlaps too much with another one
// produces noise in retrieval.
var Categories = []Category{
	{Name: "credentials", Description: "API keys, tokens, passwords, secrets, access keys"},
	{Name: "commands", Description: "CLI commands, bash one-liners, terminal shortcuts you'd reuse"},
	{Name: "snippets", Description: "Code, config, queries, and other small reusable text blocks"},
	{Name: "decisions", Description: "Architecture / tech / process decisions and the reasoning behind them"},
	{Name: "runbooks", Description: "Multi-step procedures, how-tos, deploy / incident playbooks"},
	{Name: "learnings", Description: "TILs, gotchas, debugging insights, lessons from incidents"},
	{Name: "references", Description: "URLs, documentation links, dashboards, resource pointers"},
	{Name: "contacts", Description: "People, emails, Slack handles, on-call rotations"},
	{Name: "notes", Description: "General notes, todos, reminders, anything that doesn't fit above"},
}

// FallbackCategory is used when the LLM returns a category not in the
// predefined set. "notes" is intentionally chosen as the catch-all.
const FallbackCategory = "notes"

// IsValidCategory reports whether name is one of the predefined categories.
// The check is case-insensitive but exact otherwise — partial matches are
// rejected (e.g. "credential" → false).
func IsValidCategory(name string) bool {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, c := range Categories {
		if c.Name == want {
			return true
		}
	}
	return false
}

type Category struct {
	Name        string
	Description string
}
