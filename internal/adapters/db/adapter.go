package db

import (
	"context"

	"github.com/shahid-io/inode/internal/model"
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

	// Close releases the database connection.
	Close() error
}
