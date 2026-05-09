package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/shahid-io/inode/internal/model"
)

// newClaudeWithMock points the Anthropic SDK at a httptest server. The SDK
// supports option.WithBaseURL out of the box, so no transport surgery
// needed.
func newClaudeWithMock(t *testing.T, h http.HandlerFunc) *ClaudeAdapter {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &ClaudeAdapter{
		client: anthropic.NewClient(
			option.WithAPIKey("test-key"),
			option.WithBaseURL(srv.URL),
		),
		model: anthropic.Model("claude-sonnet-4-6"),
	}
}

// writeClaudeMessage emits a Messages API response containing a single
// text block with the given content. Matches the real wire format closely
// enough for the SDK to deserialise.
func writeClaudeMessage(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":   "msg_test",
		"type": "message",
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
		"model":       "claude-sonnet-4-6",
		"stop_reason": "end_turn",
		"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
	})
}

func TestClaude_Classify_ParsesJSONResponse(t *testing.T) {
	a := newClaudeWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		writeClaudeMessage(w, `{"category":"credentials","tags":["stripe"],"is_sensitive":true,"summary":"stripe key"}`)
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
	if got.Summary != "stripe key" {
		t.Errorf("Summary = %q", got.Summary)
	}
}

func TestClaude_Classify_StripsMarkdownFences(t *testing.T) {
	// Some Claude responses arrive wrapped in ```json fences. The
	// extractJSON helper must strip them before unmarshalling.
	a := newClaudeWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		writeClaudeMessage(w, "```json\n{\"category\":\"notes\",\"tags\":[],\"is_sensitive\":false,\"summary\":\"x\"}\n```")
	})

	got, err := a.Classify(context.Background(), "x", []model.Category{{Name: "notes"}}, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if got.Category != "notes" {
		t.Errorf("Category = %q", got.Category)
	}
}

func TestClaude_Answer_NoNotes_ShortCircuits(t *testing.T) {
	called := false
	a := newClaudeWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	got, err := a.Answer(context.Background(), "anything", nil)
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if called {
		t.Error("Answer with empty notes should short-circuit before hitting Claude API")
	}
	if got.Matched {
		t.Error("expected Matched=false for empty-notes case")
	}
}

func TestClaude_Answer_StructuredOutput(t *testing.T) {
	a := newClaudeWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		writeClaudeMessage(w, `{"matched":true,"answer":"sk_test_xxx","used_note_ids":["a1f4d9c2"]}`)
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
	if len(got.UsedNoteIDs) != 1 {
		t.Errorf("UsedNoteIDs = %v", got.UsedNoteIDs)
	}
}

func TestClaude_Answer_SendsQueryAndContext(t *testing.T) {
	var sawBody string
	a := newClaudeWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		sawBody = string(body)
		writeClaudeMessage(w, `{"matched":false,"answer":"none","used_note_ids":[]}`)
	})

	notes := []*model.Note{
		{ID: "deadbeef-1111", Summary: "n", Category: "notes", ContentPlain: "needle in haystack"},
	}
	_, err := a.Answer(context.Background(), "find the needle", notes)
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}

	// The prompt should carry both the query and the note's context (or its
	// short id). Either signal proves the adapter is wiring inputs to the
	// model rather than swallowing them.
	if !strings.Contains(sawBody, "find the needle") {
		t.Errorf("request body should contain query: %s", sawBody)
	}
	if !strings.Contains(sawBody, "deadbeef") {
		t.Errorf("request body should contain note short id: %s", sawBody)
	}
}

func TestClaude_Summarize_TrimsWhitespace(t *testing.T) {
	a := newClaudeWithMock(t, func(w http.ResponseWriter, r *http.Request) {
		writeClaudeMessage(w, "  one-line summary  \n")
	})

	got, err := a.Summarize(context.Background(), "long content")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if got != "one-line summary" {
		t.Errorf("Summarize should trim, got %q", got)
	}
}
