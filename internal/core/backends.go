package core

import (
	"fmt"

	"github.com/BiSemaphore/inode/internal/adapters/embedding"
	"github.com/BiSemaphore/inode/internal/adapters/llm"
	"github.com/BiSemaphore/inode/internal/config"
)

// ResolveBackends returns a copy of cfg with model, base URL and embedding
// dimension filled in from the selected backends' defaults wherever they are
// not set. Values set in the config file or environment always win. When only
// the dimension cannot be resolved, the error comes with the config resolved
// as far as possible, for `inode config show`.
//
// Before #31 these defaults were global and Ollama-specific, so switching to
// openai, claude-api or voyage kept llama3.2, localhost:11434 and 768.
func ResolveBackends(cfg *config.Config) (*config.Config, error) {
	out := *cfg
	l, e := &out.LLM, &out.Embedding

	switch l.Backend {
	case "ollama", "":
		l.BaseURL = or(l.BaseURL, llm.OllamaDefaultURL)
		l.Model = or(l.Model, llm.OllamaDefaultModel)
	case "openai":
		l.BaseURL = or(l.BaseURL, llm.OpenAIDefaultURL)
		l.Model = or(l.Model, llm.OpenAIDefaultModel)
	case "claude-api":
		l.Model = or(l.Model, llm.ClaudeDefaultModel)
	default:
		return nil, fmt.Errorf("unknown llm.backend %q (supported: ollama, claude-api, openai)", l.Backend)
	}

	switch e.Backend {
	case "ollama", "":
		e.BaseURL = or(e.BaseURL, embedding.OllamaDefaultURL)
		e.Model = or(e.Model, embedding.OllamaDefaultModel)
	case "voyage":
		e.Model = or(e.Model, embedding.VoyageDefaultModel)
	default:
		return nil, fmt.Errorf("unknown embedding.backend %q (supported: ollama, voyage)", e.Backend)
	}

	if e.Dimension == 0 {
		d, ok := embedding.Dimension(e.Model)
		if !ok {
			return &out, fmt.Errorf("embedding.dimension is not set and inode does not know the vector size of %q; set it, for example: inode config set embedding.dimension 1024", e.Model)
		}
		e.Dimension = d
	}
	return &out, nil
}

func or(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
