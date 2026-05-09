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

// newVoyageWithMock returns a VoyageAdapter pointed at a test server. The
// server runs the supplied handler and the returned cleanup closes it.
func newVoyageWithMock(t *testing.T, h http.HandlerFunc) (*VoyageAdapter, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	a := NewVoyageAdapter("test-key", "voyage-3")
	// Override the production URL via the unexported field. We swap the
	// constant with the test URL using a tiny indirection: NewVoyageAdapter
	// uses voyageAPIURL, so we redirect by replacing apiKey usage isn't
	// enough — use the http.Client's transport instead.
	a.client = &http.Client{Transport: &redirectingTransport{target: srv.URL}}
	return a, srv
}

// redirectingTransport rewrites every outgoing request to point at a fixed
// host while preserving the path. Used so Voyage / Ollama tests don't
// require exposing the production URL constants.
type redirectingTransport struct{ target string }

func (r *redirectingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace scheme + host with the test server's, keep path.
	u := *req.URL
	parsed, _ := http.NewRequest(req.Method, r.target+u.Path, req.Body)
	parsed.Header = req.Header.Clone()
	return http.DefaultTransport.RoundTrip(parsed)
}

func TestVoyage_Embed_SendsCorrectRequest(t *testing.T) {
	var sawBody string
	var sawAuth string
	a, _ := newVoyageWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		sawBody = string(body)
		sawAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(voyageResponse{
			Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{{Embedding: []float32{0.1, 0.2, 0.3}, Index: 0}},
		})
	})

	_, err := a.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	if sauth := sawAuth; sauth != "Bearer test-key" {
		t.Errorf("Authorization header = %q, want Bearer test-key", sauth)
	}
	if !strings.Contains(sawBody, `"input":["hello world"]`) {
		t.Errorf("request body missing input: %s", sawBody)
	}
	if !strings.Contains(sawBody, `"model":"voyage-3"`) {
		t.Errorf("request body missing model: %s", sawBody)
	}
}

func TestVoyage_Embed_ParsesSuccessResponse(t *testing.T) {
	want := []float32{0.5, 0.25, 0.125, 0.0625}
	a, _ := newVoyageWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(voyageResponse{
			Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{{Embedding: want, Index: 0}},
		})
	})

	got, err := a.Embed(context.Background(), "x")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("vector length mismatch: got %d, want %d", len(got), len(want))
	}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("vector[%d] = %v, want %v", i, v, want[i])
		}
	}
}

func TestVoyage_Embed_PropagatesAPIErrorMessage(t *testing.T) {
	a, _ := newVoyageWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": {"message": "invalid api key"}}`))
	})

	_, err := a.Embed(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
	if !strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("error should include API message, got: %v", err)
	}
}

func TestVoyage_Embed_HandlesEmptyData(t *testing.T) {
	a, _ := newVoyageWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": []}`))
	})

	_, err := a.Embed(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error for empty data array")
	}
}
