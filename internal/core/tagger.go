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

	// Reject hallucinated categories. The LLM is asked to pick from a fixed
	// list, but small local models occasionally invent ones that are close
	// (e.g. "credential", "command-line", "code"). Anything not in the
	// predefined set falls back to FallbackCategory rather than being
	// silently persisted and creating retrieval noise later.
	result.Category = normalise(result.Category)
	if !model.IsValidCategory(result.Category) {
		result.Category = model.FallbackCategory
	}

	// User overrides take precedence over LLM output. We accept whatever
	// the user typed — they may genuinely want a custom category, in which
	// case the validation isn't ours to enforce.
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

// defaultTags returns the built-in suggested tags sent to the LLM as
// classification hints. The LLM is told to use these where they fit and
// invent new ones otherwise — so this list is a soft vocabulary, not a
// hard constraint.
//
// Curated to cover the common dev-tool landscape without becoming
// unwieldy. Adding a tag here biases the LLM toward picking it for
// future notes, which improves consistency across a knowledge base.
func defaultTags() []string {
	return []string{
		// clouds + infra
		"aws", "gcp", "azure", "docker", "kubernetes", "terraform", "helm",
		// databases
		"postgres", "mysql", "sqlite", "redis", "mongodb", "elasticsearch",
		// languages
		"go", "python", "rust", "typescript", "javascript", "bash", "sql",
		// frameworks
		"react", "vue", "next", "express", "django", "fastapi",
		// version control + CI
		"git", "github", "gitlab", "ci", "cd",
		// services
		"stripe", "twilio", "slack", "linear", "datadog", "sentry", "cloudflare",
		// secrets / auth
		"api", "token", "password", "secret", "key", "oauth", "jwt", "ssh",
		// environments
		"prod", "staging", "dev", "local",
		// concerns
		"backup", "monitoring", "logging", "performance", "security", "incident",
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
