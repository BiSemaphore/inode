package embedding

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newOllamaWithMock(t *testing.T, h http.HandlerFunc) *OllamaAdapter {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewOllamaEmbeddingAdapter(srv.URL, "nomic-embed-text")
}

func TestOllamaEmbed_SendsCorrectRequest(t *testing.T) {
	var sawPath, sawBody string
	a := newOllamaWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		sawBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embeddings": [[0.1, 0.2, 0.3]]}`))
	})

	_, err := a.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	if sawPath != "/api/embed" {
		t.Errorf("path = %q, want /api/embed", sawPath)
	}
	if !strings.Contains(sawBody, `"input":"hello"`) {
		t.Errorf("body missing input: %s", sawBody)
	}
	if !strings.Contains(sawBody, `"model":"nomic-embed-text"`) {
		t.Errorf("body missing model: %s", sawBody)
	}
}

func TestOllamaEmbed_ParsesSuccessResponse(t *testing.T) {
	want := []float32{0.5, 0.25, 0.125}
	a := newOllamaWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ollamaEmbedResponse{
			Embeddings: [][]float32{want},
		})
	})

	got, err := a.Embed(context.Background(), "x")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("vector[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestOllamaEmbed_HandlesNon200(t *testing.T) {
	a := newOllamaWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	_, err := a.Embed(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error for 503 response")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error should mention status code, got: %v", err)
	}
}

func TestOllamaEmbed_HandlesEmptyEmbeddings(t *testing.T) {
	a := newOllamaWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"embeddings": []}`))
	})

	_, err := a.Embed(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error for empty embeddings array")
	}
}

func TestOllamaEmbed_DefaultsBaseURL(t *testing.T) {
	// When constructed with empty baseURL, NewOllamaEmbeddingAdapter falls
	// back to localhost:11434. We don't make a request — just check the
	// fallback is wired.
	a := NewOllamaEmbeddingAdapter("", "")
	if a.baseURL != "http://localhost:11434" {
		t.Errorf("default baseURL = %q", a.baseURL)
	}
	if a.model != "nomic-embed-text" {
		t.Errorf("default model = %q", a.model)
	}
}
