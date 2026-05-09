package embedding

import "context"

// Adapter defines the contract for all embedding backends.
// Implementations: voyage.go, ollama.go.
//
// Implementations MUST return L2-normalised vectors (‖v‖₂ = 1). The search
// layer assumes this when interpreting distance scores — an L2 distance
// threshold of 1.0 corresponds to cos_sim ≈ 0.5 only for unit vectors. Both
// Voyage AI and Ollama nomic-embed-text return normalised vectors by default;
// any new adapter that does not must normalise before returning.
//
// All embeddings in the DB must use the same model and dimensions.
type Adapter interface {
	// Embed converts text to a normalised float32 vector.
	Embed(ctx context.Context, text string) ([]float32, error)
}
