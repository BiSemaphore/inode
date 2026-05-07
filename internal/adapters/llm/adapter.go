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

// Adapter defines the contract for all LLM backends.
// Implementations: claude.go (Phase 1), ollama.go (Phase 3).
type Adapter interface {
	// Classify returns category, tags, sensitivity, and a one-line summary.
	// Called when a note is added without explicit metadata.
	Classify(ctx context.Context, content string, categories []model.Category, tags []string) (ClassifyResult, error)

	// Answer performs RAG generation: query + retrieved notes → natural language response.
	Answer(ctx context.Context, query string, notes []*model.Note) (string, error)

	// Summarize returns a one-line description of the note content.
	Summarize(ctx context.Context, content string) (string, error)
}
