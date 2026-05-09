package llm

import (
	"context"

	"github.com/shahid-io/inode/internal/model"
)

// ClassifyResult is the structured output from LLM classification.
type ClassifyResult struct {
	Category    string
	Tags        []string
	IsSensitive bool
	Summary     string
}

// AnswerResult is the structured output from RAG answer generation.
//
// Matched is true iff the LLM concluded that at least one of the provided
// notes contained the answer. UsedNoteIDs holds short-prefix IDs of the
// notes the LLM actually used; the search layer filters the retrieved
// notes down to these so the CLI only displays sources that contributed.
//
// When Matched is false, UsedNoteIDs is empty and Answer is the LLM's
// natural-language explanation that the answer is not in the notes.
type AnswerResult struct {
	Answer      string
	Matched     bool
	UsedNoteIDs []string
}

// Adapter defines the contract for all LLM backends.
// Implementations: claude.go, ollama.go.
type Adapter interface {
	// Classify returns category, tags, sensitivity, and a one-line summary.
	// Called when a note is added without explicit metadata.
	Classify(ctx context.Context, content string, categories []model.Category, tags []string) (ClassifyResult, error)

	// Answer performs RAG generation: query + retrieved notes → structured response.
	// The response carries both the natural-language answer and a flag telling the
	// caller whether the notes were actually useful (so callers can hide "Sources"
	// blocks when the LLM rejected every candidate).
	Answer(ctx context.Context, query string, notes []*model.Note) (AnswerResult, error)

	// Summarize returns a one-line description of the note content.
	Summarize(ctx context.Context, content string) (string, error)
}
