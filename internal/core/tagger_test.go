package core

import (
	"context"
	"testing"

	"github.com/shahid-io/inode/internal/adapters/llm"
	"github.com/shahid-io/inode/internal/model"
)

// stubLLM is the minimal adapter needed to drive TaggerService — only
// Classify is exercised. Other methods return empty values.
type stubLLM struct {
	classify llm.ClassifyResult
}

func (s *stubLLM) Classify(_ context.Context, _ string, _ []model.Category, _ []string) (llm.ClassifyResult, error) {
	return s.classify, nil
}

func (s *stubLLM) Answer(_ context.Context, _ string, _ []*model.Note) (llm.AnswerResult, error) {
	return llm.AnswerResult{}, nil
}

func (s *stubLLM) Summarize(_ context.Context, _ string) (string, error) {
	return "", nil
}

func TestTaggerClassify_HallucinatedCategoryFallsBackToNotes(t *testing.T) {
	stub := &stubLLM{classify: llm.ClassifyResult{
		Category:    "command-line", // close to "commands" but not in Categories
		Tags:        []string{"bash"},
		IsSensitive: false,
		Summary:     "test",
	}}
	tagger := NewTaggerService(stub)

	got, err := tagger.Classify(context.Background(), "echo hello", ClassifyOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Category != model.FallbackCategory {
		t.Fatalf("expected fallback %q for hallucinated category, got %q",
			model.FallbackCategory, got.Category)
	}
}

func TestTaggerClassify_ValidLLMCategoryPassesThrough(t *testing.T) {
	stub := &stubLLM{classify: llm.ClassifyResult{
		Category: "credentials",
		Tags:     []string{"stripe"},
		Summary:  "test",
	}}
	tagger := NewTaggerService(stub)

	got, err := tagger.Classify(context.Background(), "sk_test_xxx", ClassifyOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Category != "credentials" {
		t.Fatalf("expected category passthrough, got %q", got.Category)
	}
}

func TestTaggerClassify_UserCategoryOverrideBypassesValidation(t *testing.T) {
	// The user is trusted to pick their own categories — even non-canonical
	// ones — when they explicitly supply --category. Only the LLM is
	// validated against Categories.
	stub := &stubLLM{classify: llm.ClassifyResult{
		Category: "credentials",
		Tags:     []string{"stripe"},
		Summary:  "test",
	}}
	tagger := NewTaggerService(stub)

	got, err := tagger.Classify(context.Background(), "x", ClassifyOptions{
		Category: "my-custom-bucket",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Category != "my-custom-bucket" {
		t.Fatalf("expected user override to pass through, got %q", got.Category)
	}
}

func TestTaggerClassify_LLMValidatedCategoryNormalised(t *testing.T) {
	// Casing and whitespace from the LLM should be normalised before the
	// validity check (e.g. "Credentials" should match "credentials").
	stub := &stubLLM{classify: llm.ClassifyResult{
		Category: "  Credentials  ",
		Summary:  "test",
	}}
	tagger := NewTaggerService(stub)

	got, err := tagger.Classify(context.Background(), "x", ClassifyOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Category != "credentials" {
		t.Fatalf("expected normalised category, got %q", got.Category)
	}
}
