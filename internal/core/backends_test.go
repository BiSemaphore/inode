package core

import (
	"strings"
	"testing"

	"github.com/BiSemaphore/inode/internal/config"
)

// Regression for #31: cloud backends inherited Ollama's model, base URL and
// dimension because those were global defaults.
func TestResolveBackends(t *testing.T) {
	tests := []struct {
		name string
		in   config.Config
		want config.Config
	}{
		{
			name: "defaults unchanged: ollama for both",
			in:   config.Config{},
			want: config.Config{
				LLM:       config.LLMConfig{Model: "llama3.2", BaseURL: "http://localhost:11434"},
				Embedding: config.EmbeddingConfig{Model: "nomic-embed-text", BaseURL: "http://localhost:11434", Dimension: 768},
			},
		},
		{
			name: "openai uses its own URL and model",
			in:   config.Config{LLM: config.LLMConfig{Backend: "openai"}},
			want: config.Config{
				LLM:       config.LLMConfig{Backend: "openai", Model: "gpt-4o-mini", BaseURL: "https://api.openai.com/v1"},
				Embedding: config.EmbeddingConfig{Model: "nomic-embed-text", BaseURL: "http://localhost:11434", Dimension: 768},
			},
		},
		{
			name: "claude-api uses its own model and no base URL",
			in:   config.Config{LLM: config.LLMConfig{Backend: "claude-api"}},
			want: config.Config{
				LLM:       config.LLMConfig{Backend: "claude-api", Model: "claude-sonnet-5-5"},
				Embedding: config.EmbeddingConfig{Model: "nomic-embed-text", BaseURL: "http://localhost:11434", Dimension: 768},
			},
		},
		{
			name: "voyage uses voyage-3 and 1024 dimensions",
			in:   config.Config{Embedding: config.EmbeddingConfig{Backend: "voyage"}},
			want: config.Config{
				LLM:       config.LLMConfig{Model: "llama3.2", BaseURL: "http://localhost:11434"},
				Embedding: config.EmbeddingConfig{Backend: "voyage", Model: "voyage-3", Dimension: 1024},
			},
		},
		{
			name: "values set by the user win for any backend",
			in: config.Config{
				LLM:       config.LLMConfig{Backend: "openai", Model: "gpt-4.1", BaseURL: "https://example.openai.azure.com/v1"},
				Embedding: config.EmbeddingConfig{Backend: "ollama", Model: "mxbai-embed-large", BaseURL: "http://gpu-box:11434", Dimension: 1024},
			},
			want: config.Config{
				LLM:       config.LLMConfig{Backend: "openai", Model: "gpt-4.1", BaseURL: "https://example.openai.azure.com/v1"},
				Embedding: config.EmbeddingConfig{Backend: "ollama", Model: "mxbai-embed-large", BaseURL: "http://gpu-box:11434", Dimension: 1024},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveBackends(&tt.in)
			if err != nil {
				t.Fatalf("ResolveBackends: %v", err)
			}
			if got.LLM != tt.want.LLM {
				t.Errorf("LLM = %+v, want %+v", got.LLM, tt.want.LLM)
			}
			if got.Embedding != tt.want.Embedding {
				t.Errorf("Embedding = %+v, want %+v", got.Embedding, tt.want.Embedding)
			}
		})
	}
}

func TestResolveBackendsDoesNotModifyInput(t *testing.T) {
	in := config.Config{LLM: config.LLMConfig{Backend: "openai"}}
	if _, err := ResolveBackends(&in); err != nil {
		t.Fatal(err)
	}
	if in.LLM.Model != "" || in.LLM.BaseURL != "" || in.Embedding.Dimension != 0 {
		t.Errorf("input was modified: %+v", in)
	}
}

func TestResolveBackendsErrors(t *testing.T) {
	tests := []struct {
		name    string
		in      config.Config
		wantErr string
	}{
		{"unknown model needs a dimension",
			config.Config{Embedding: config.EmbeddingConfig{Model: "mxbai-embed-large"}},
			`embedding.dimension is not set and inode does not know the vector size of "mxbai-embed-large"`},
		{"unknown llm backend",
			config.Config{LLM: config.LLMConfig{Backend: "gemini"}},
			`unknown llm.backend "gemini"`},
		{"unknown embedding backend",
			config.Config{Embedding: config.EmbeddingConfig{Backend: "openai"}},
			`unknown embedding.backend "openai"`},
	}
	for _, tt := range tests {
		if _, err := ResolveBackends(&tt.in); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("%s: error = %v, want %q", tt.name, err, tt.wantErr)
		}
	}
}
