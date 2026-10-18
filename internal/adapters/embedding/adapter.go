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

// Defaults used by each backend when embedding.model or embedding.base_url is
// not set. core.ResolveBackends applies them.
const (
	OllamaDefaultURL   = "http://localhost:11434"
	OllamaDefaultModel = "nomic-embed-text"
	VoyageDefaultModel = "voyage-3"
)

// knownDimensions are the vector sizes of the default models. Any other model
// needs embedding.dimension set explicitly: a wrong guess would create the
// vector table with the wrong size.
var knownDimensions = map[string]int{
	OllamaDefaultModel: 768,
	VoyageDefaultModel: 1024,
}

// Dimension returns the vector size of a known embedding model.
func Dimension(model string) (int, bool) {
	d, ok := knownDimensions[model]
	return d, ok
}
