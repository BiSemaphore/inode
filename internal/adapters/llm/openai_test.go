package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BiSemaphore/inode/internal/model"
)

func newOpenAILLMWithMock(t *testing.T, h http.HandlerFunc) *OpenAIAdapter {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewOpenAIAdapter("sk-test", "gpt-4o-mini", srv.URL)
}

// helper to write a chat completion with the given message content
func writeOpenAIChat(w http.ResponseWriter, content string) {
	resp := openAIChatResponse{}
	resp.Choices = append(resp.Choices, struct {
		Message openAIMessage `json:"message"`
	}{Message: openAIMessage{Role: "assistant", Content: content}})
	_ = json.NewEncoder(w).Encode(resp)
}

func TestNewOpenAIAdapter_Defaults(t *testing.T) {
	a := NewOpenAIAdapter("sk-test", "", "")
	if a.baseURL != "https://api.openai.com/v1" {
		t.Errorf("baseURL = %q", a.baseURL)
	}
	if a.model != "gpt-4o-mini" {
		t.Errorf("model = %q", a.model)
	}
}

func TestOpenAILLM_Classify_RequestShape(t *testing.T) {
	var sawPath, sawAuth, sawBody string
	a := newOpenAILLMWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		sawAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		sawBody = string(body)
		writeOpenAIChat(w, `{"category":"commands","tags":["bash"],"is_sensitive":false,"summary":"echo hello"}`)
	})

	cats := []model.Category{{Name: "commands", Description: "CLI"}}
	if _, err := a.Classify(context.Background(), "echo hello", cats, nil); err != nil {
		t.Fatalf("Classify: %v", err)
	}

	if sawPath != "/chat/completions" {
		t.Errorf("path = %q, want /chat/completions", sawPath)
	}
	if sawAuth != "Bearer sk-test" {
		t.Errorf("Authorization = %q", sawAuth)
	}
	if !strings.Contains(sawBody, `"response_format":{"type":"json_object"}`) {
		t.Errorf("expected JSON mode for Classify, body: %s", sawBody)
	}
}

func TestOpenAILLM_Classify_ParsesResponse(t *testing.T) {
	a := newOpenAILLMWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		writeOpenAIChat(w, `{"category":"credentials","tags":["stripe","payment"],"is_sensitive":true,"summary":"stripe key"}`)
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

func TestOpenAILLM_Answer_NoNotes_ShortCircuits(t *testing.T) {
	// The mock should never be hit when notes is empty.
	called := false
	a := newOpenAILLMWithMock(t, func(w http.ResponseWriter, r *http.Request) {
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

func TestOpenAILLM_Answer_StructuredOutput(t *testing.T) {
	a := newOpenAILLMWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		writeOpenAIChat(w, `{"matched":true,"answer":"sk_test_xxx","used_note_ids":["a1f4d9c2"]}`)
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

func TestOpenAILLM_APIError_Surfaces(t *testing.T) {
	a := newOpenAILLMWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Incorrect API key provided"}}`))
	})

	_, err := a.Summarize(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "Incorrect API key") {
		t.Errorf("error should carry status and API message, got: %v", err)
	}
}

func TestOpenAILLM_Summarize_NoJSONMode(t *testing.T) {
	var sawBody string
	a := newOpenAILLMWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		sawBody = string(body)
		writeOpenAIChat(w, "  Prints hello to stdout.  ")
	})

	got, err := a.Summarize(context.Background(), "echo hello")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if got != "Prints hello to stdout." {
		t.Errorf("Summarize = %q, want trimmed sentence", got)
	}
	if strings.Contains(sawBody, "response_format") {
		t.Errorf("Summarize should not request JSON mode, body: %s", sawBody)
	}
}
