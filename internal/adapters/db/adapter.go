package db

import (
	"context"

	"github.com/BiSemaphore/inode/internal/model"
)

// Filters for list and search queries.
type Filters struct {
	Category    string
	Tags        []string
	IsSensitive *bool // nil = no filter
}

// Adapter defines the contract for all database backends.
// Implementations: sqlite.go (Phase 1), postgres.go (Phase 2).
type Adapter interface {
	// Save persists a note with its embedding. Returns the assigned ID.
	Save(ctx context.Context, note *model.Note) (string, error)

	// Get fetches a single note by ID.
	Get(ctx context.Context, id string) (*model.Note, error)

	// Delete removes a note by ID.
	Delete(ctx context.Context, id string) error

	// SearchSimilar returns the top-K notes by cosine similarity to vec,
	// optionally filtered by Filters.
	SearchSimilar(ctx context.Context, vec []float32, topK int, filters Filters) ([]*model.Note, error)

	// List returns notes matching filters, with pagination.
	List(ctx context.Context, filters Filters, limit, offset int) ([]*model.Note, error)

	// UncheckedSensitive returns sensitive notes whose summary has not yet
	// been validated against their content (notes saved before v0.5.0).
	UncheckedSensitive(ctx context.Context) ([]*model.Note, error)

	// SetSummary replaces a note's summary by full ID and marks it validated.
	SetSummary(ctx context.Context, id, summary string) error

	// Close releases the database connection.
	Close() error
}
