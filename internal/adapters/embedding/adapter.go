package embedding

import "context"

// Adapter defines the contract for all embedding backends.
// Implementations: voyage.go (Phase 1), local.go (Phase 3).
type Adapter interface {
	// Embed converts text to a float32 vector.
	// All embeddings in the DB must use the same model and dimensions.
	// voyage-3 produces 1024-dimensional vectors.
	Embed(ctx context.Context, text string) ([]float32, error)
}
