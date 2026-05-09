package llm

import (
	"testing"
)

func TestParseAnswerJSON_MatchedWithIDs(t *testing.T) {
	raw := `{"matched": true, "answer": "your stripe key is sk_test_xxx", "used_note_ids": ["a1f4d9c2"]}`
	got, err := parseAnswerJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Matched {
		t.Fatalf("expected Matched=true, got false")
	}
	if got.Answer != "your stripe key is sk_test_xxx" {
		t.Fatalf("answer mismatch: %q", got.Answer)
	}
	if len(got.UsedNoteIDs) != 1 || got.UsedNoteIDs[0] != "a1f4d9c2" {
		t.Fatalf("UsedNoteIDs mismatch: %v", got.UsedNoteIDs)
	}
}

func TestParseAnswerJSON_NotMatched(t *testing.T) {
	raw := `{"matched": false, "answer": "the answer is not in these notes.", "used_note_ids": []}`
	got, err := parseAnswerJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Matched {
		t.Fatalf("expected Matched=false")
	}
	if len(got.UsedNoteIDs) != 0 {
		t.Fatalf("expected empty UsedNoteIDs, got %v", got.UsedNoteIDs)
	}
}

func TestParseAnswerJSON_MatchedTrueButNoIDs_CoercedToNotMatched(t *testing.T) {
	// Defensive: model claims match but provides no IDs. Treat as not matched.
	raw := `{"matched": true, "answer": "kind of?", "used_note_ids": []}`
	got, err := parseAnswerJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Matched {
		t.Fatalf("expected coerced Matched=false when no IDs provided")
	}
}

func TestParseAnswerJSON_WithMarkdownFences(t *testing.T) {
	// Some models wrap JSON in ```json fences. extractJSON should strip them.
	raw := "```json\n{\"matched\": true, \"answer\": \"ok\", \"used_note_ids\": [\"abc12345\"]}\n```"
	got, err := parseAnswerJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Matched || got.Answer != "ok" || got.UsedNoteIDs[0] != "abc12345" {
		t.Fatalf("parse with fences failed: %+v", got)
	}
}

func TestParseAnswerJSON_EmptyAnswer_GetsFallback(t *testing.T) {
	raw := `{"matched": false, "answer": "", "used_note_ids": []}`
	got, err := parseAnswerJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Answer == "" {
		t.Fatalf("expected fallback answer when LLM returned empty string")
	}
}

func TestParseAnswerJSON_MalformedJSON_Errors(t *testing.T) {
	raw := `not actually json at all`
	_, err := parseAnswerJSON(raw)
	if err == nil {
		t.Fatalf("expected error on malformed JSON")
	}
}
