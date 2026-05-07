package core

import (
	"context"
	"strings"

	"github.com/shahid-io/inode/internal/adapters/llm"
	"github.com/shahid-io/inode/internal/model"
)

// TaggerService classifies notes into categories and tags.
// When the user provides values explicitly, they are passed through (with normalisation).
// Otherwise the LLM adapter is called for auto-detection.
type TaggerService struct {
	llm llm.Adapter
}

// NewTaggerService creates a TaggerService backed by the given LLM adapter.
func NewTaggerService(llmAdapter llm.Adapter) *TaggerService {
	return &TaggerService{llm: llmAdapter}
}

// ClassifyOptions carries optional user-supplied overrides.
type ClassifyOptions struct {
	Category    string   // explicit category; empty = auto-detect
	Tags        []string // explicit tags; nil = auto-detect
	IsSensitive *bool    // explicit sensitivity; nil = auto-detect
}

// Classify returns the final classification for a note.
// User-supplied values take precedence; the LLM fills in anything missing.
func (t *TaggerService) Classify(ctx context.Context, content string, opts ClassifyOptions) (llm.ClassifyResult, error) {
	// If everything is provided by the user, skip the LLM call.
	if opts.Category != "" && len(opts.Tags) > 0 && opts.IsSensitive != nil {
		summary, err := t.llm.Summarize(ctx, content)
		if err != nil {
			summary = firstLine(content, 80)
		}
		return llm.ClassifyResult{
			Category:    normalise(opts.Category),
			Tags:        normaliseTags(opts.Tags),
			IsSensitive: *opts.IsSensitive,
			Summary:     summary,
		}, nil
	}

	// Call LLM for auto-detection.
	result, err := t.llm.Classify(ctx, content, model.Categories, defaultTags())
	if err != nil {
		return llm.ClassifyResult{}, err
	}

	// User overrides take precedence over LLM output.
	if opts.Category != "" {
		result.Category = normalise(opts.Category)
	}
	if len(opts.Tags) > 0 {
		result.Tags = normaliseTags(opts.Tags)
	}
	if opts.IsSensitive != nil {
		result.IsSensitive = *opts.IsSensitive
	}

	return result, nil
}

// defaultTags returns the built-in suggested tags sent to the LLM as hints.
func defaultTags() []string {
	return []string{
		"aws", "gcp", "azure", "github", "gitlab", "stripe", "twilio",
		"docker", "kubernetes", "terraform", "postgres", "mysql", "redis",
		"python", "golang", "nodejs", "bash", "ssh", "api", "token",
		"password", "secret", "key", "prod", "dev", "staging",
	}
}

func normalise(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func normaliseTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if v := normalise(t); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func firstLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}
