package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shahid-io/inode/internal/model"
)

func newOllamaLLMWithMock(t *testing.T, h http.HandlerFunc) *OllamaAdapter {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewOllamaAdapter(srv.URL, "llama3.2")
}

// helper to write a chat response with the given message content
func writeOllamaChat(w http.ResponseWriter, content string) {
	_ = json.NewEncoder(w).Encode(ollamaChatResponse{
		Message: ollamaMessage{Role: "assistant", Content: content},
	})
}

func TestOllamaLLM_Classify_RequestsJSONFormat(t *testing.T) {
	var sawBody string
	a := newOllamaLLMWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		sawBody = string(body)
		writeOllamaChat(w, `{"category":"commands","tags":["bash"],"is_sensitive":false,"summary":"echo hello"}`)
	})

	cats := []model.Category{{Name: "commands", Description: "CLI"}}
	_, err := a.Classify(context.Background(), "echo hello", cats, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	if !strings.Contains(sawBody, `"format":"json"`) {
		t.Errorf("expected format:json mode for Classify, body: %s", sawBody)
	}
}

func TestOllamaLLM_Classify_ParsesResponse(t *testing.T) {
	a := newOllamaLLMWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		writeOllamaChat(w, `{"category":"credentials","tags":["stripe","payment"],"is_sensitive":true,"summary":"stripe key"}`)
	})

	got, err := a.Classify(context.Background(), "sk_test_xxx", []model.Category{{Name: "credentials"}}, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if got.Category != "credentials" {
		t.Errorf("Category = %q", got.Category)
	}
	if !got.IsSensitive {
		t.Error("IsSensitive should be true")
	}
	if len(got.Tags) != 2 {
		t.Errorf("expected 2 tags, got %v", got.Tags)
	}
}

func TestOllamaLLM_Answer_NoNotes_ShortCircuits(t *testing.T) {
	// The mock should never be hit when notes is empty.
	called := false
	a := newOllamaLLMWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	got, err := a.Answer(context.Background(), "anything", nil)
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if called {
		t.Error("Answer with empty notes should short-circuit before hitting LLM")
	}
	if got.Matched {
		t.Error("expected Matched=false for empty-notes case")
	}
}

func TestOllamaLLM_Answer_StructuredOutput(t *testing.T) {
	a := newOllamaLLMWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		writeOllamaChat(w, `{"matched":true,"answer":"sk_test_xxx","used_note_ids":["a1f4d9c2"]}`)
	})

	notes := []*model.Note{
		{ID: "a1f4d9c2-1111-2222-3333-444444444444", Summary: "stripe", Category: "credentials", ContentPlain: "sk_test_xxx"},
	}
	got, err := a.Answer(context.Background(), "stripe key", notes)
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if !got.Matched {
		t.Error("expected Matched=true")
	}
	if got.Answer != "sk_test_xxx" {
		t.Errorf("Answer = %q", got.Answer)
	}
	if len(got.UsedNoteIDs) != 1 || got.UsedNoteIDs[0] != "a1f4d9c2" {
		t.Errorf("UsedNoteIDs = %v", got.UsedNoteIDs)
	}
}

func TestOllamaLLM_Answer_NotMatched(t *testing.T) {
	a := newOllamaLLMWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		writeOllamaChat(w, `{"matched":false,"answer":"the answer is not in these notes","used_note_ids":[]}`)
	})

	notes := []*model.Note{{ID: "a1f4d9c2-1111", Summary: "x", Category: "notes", ContentPlain: "x"}}
	got, err := a.Answer(context.Background(), "anything", notes)
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if got.Matched {
		t.Error("expected Matched=false")
	}
	if !strings.Contains(got.Answer, "not in these notes") {
		t.Errorf("answer should pass through model's natural-language reply, got %q", got.Answer)
	}
}

func TestOllamaLLM_Summarize(t *testing.T) {
	a := newOllamaLLMWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		writeOllamaChat(w, "  echo hello  ")
	})

	got, err := a.Summarize(context.Background(), "echo hello")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if got != "echo hello" {
		t.Errorf("Summarize should trim whitespace; got %q", got)
	}
}

func TestOllamaLLM_HandlesNon200(t *testing.T) {
	a := newOllamaLLMWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := a.Summarize(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
