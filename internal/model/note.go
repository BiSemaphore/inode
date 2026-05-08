package model

import "time"

type Note struct {
	ID           string
	UserID       string    // empty in Phase 1 (local, single-user)
	ContentEnc   []byte    // iv[12] + gcm_sealed[N]; nil if not sensitive
	ContentPlain string    // empty if sensitive
	Summary      string    // LLM-generated one-liner
	Category     string    // credentials | commands | decisions | references | notes
	Tags         []string
	IsSensitive  bool
	Embedding    []float32 // voyage-3: 1024 dims
	CreatedAt    time.Time
	UpdatedAt    time.Time

	// Distance is the L2 distance from the query vector when this note was
	// returned by a similarity search. 0 if not from a search. Not persisted.
	Distance float32
}

// Categories defines the predefined set of note categories.
var Categories = []Category{
	{Name: "credentials", Description: "API keys, tokens, passwords, secrets, access keys"},
	{Name: "commands", Description: "CLI commands, bash scripts, terminal shortcuts"},
	{Name: "decisions", Description: "Architectural decisions, tech choices, meeting outcomes"},
	{Name: "references", Description: "URLs, documentation links, resource pointers"},
	{Name: "notes", Description: "General notes, todos, reminders, ideas"},
}

type Category struct {
	Name        string
	Description string
}
