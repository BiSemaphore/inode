package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/shahid-io/inode/internal/adapters/db"
	"github.com/shahid-io/inode/internal/adapters/embedding"
	"github.com/shahid-io/inode/internal/adapters/llm"
	"github.com/shahid-io/inode/internal/model"
)

const (
	defaultTopK = 5

	// defaultMaxDistance is the L2-distance ceiling for considering a note
	// relevant to the query. Embedding vectors from Voyage and Ollama
	// nomic-embed-text are L2-normalised, so distance ∈ [0, 2]:
	//   0.0 → identical, 1.0 → cos_sim 0.5, 1.4 → orthogonal.
	// 1.0 filters out clearly off-topic matches while keeping moderately
	// related ones. Override via SearchOptions.MaxDistance or config.
	defaultMaxDistance = 1.0
)

// SearchResult holds both the LLM-generated answer and the raw notes used as context.
type SearchResult struct {
	Answer string
	Notes  []*model.Note
}

// SearchService implements the RAG pipeline for natural language queries.
// All external concerns are pluggable via adapters — Voyage/Ollama for
// embeddings, Claude/Ollama for the LLM, SQLite/(future)Postgres for storage.
//
// Flow:
//  1. Embed the user query (embedding adapter)
//  2. Vector similarity search → top-K notes (DB adapter)
//  3. Drop notes beyond the relevance threshold
//  4. Decrypt sensitive notes in memory
//  5. Inject remaining notes as context → answer (LLM adapter)
type SearchService struct {
	db          db.Adapter
	embedding   embedding.Adapter
	llm         llm.Adapter
	keyMgr      *KeyManager
	maxDistance float32 // service-level default; per-call override via SearchOptions
	topK        int     // service-level default; per-call override via SearchOptions
}

// NewSearchService creates a SearchService with all required dependencies.
// maxDistance and topK supply service-level defaults; pass 0 to use built-ins.
func NewSearchService(dbAdapter db.Adapter, embAdapter embedding.Adapter, llmAdapter llm.Adapter, keyMgr *KeyManager, maxDistance float32, topK int) *SearchService {
	if maxDistance == 0 {
		maxDistance = defaultMaxDistance
	}
	if topK <= 0 {
		topK = defaultTopK
	}
	return &SearchService{
		db:          dbAdapter,
		embedding:   embAdapter,
		llm:         llmAdapter,
		keyMgr:      keyMgr,
		maxDistance: maxDistance,
		topK:        topK,
	}
}

// SearchOptions carries optional filters for a search query.
type SearchOptions struct {
	TopK     int
	Category string
	Tags     []string

	// IsSensitive optionally restricts results by the sensitive flag.
	// nil = no filter (default); pointer to false = exclude sensitive
	// notes from results before they reach the LLM. Used by the MCP
	// server to prevent sensitive notes leaking to the calling agent.
	IsSensitive *bool

	// MaxDistance is the L2-distance ceiling for keeping a candidate note.
	//
	//   = 0  →  use the service default (typically 1.0)
	//   > 0  →  keep notes with Distance <= MaxDistance
	//   < 0  →  disable filtering entirely (return whatever the DB returned)
	//
	// The "= 0 means default" overload means an *exact* 0 threshold cannot be
	// expressed via this field. In practice this is fine: with L2-normalised
	// embeddings, bit-exact distance 0 implies bit-identical content, which is
	// not a useful retrieval target.
	MaxDistance float32

	// OnStep is an optional progress hook invoked at the start of each
	// pipeline phase ("embedding", "searching", "answering"). Used by the
	// CLI to update a spinner label. Must be cheap and non-blocking.
	OnStep func(step string)
}

// Retrieve runs the embed → vector search → threshold filter → decrypt
// pipeline and returns the candidate notes. No LLM call.
//
// Used by the MCP tool surface: the calling agent (Claude Code, Cursor)
// is itself an LLM and prefers to reason over raw candidates rather than
// a pre-generated answer from inode's local model.
func (s *SearchService) Retrieve(ctx context.Context, query string, opts SearchOptions) ([]*model.Note, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	topK := opts.TopK
	if topK <= 0 {
		topK = s.topK
	}

	step := opts.OnStep
	if step == nil {
		step = func(string) {}
	}

	step("embedding query")
	vec, err := s.embedding.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	step("searching")
	filters := db.Filters{
		Category:    opts.Category,
		Tags:        opts.Tags,
		IsSensitive: opts.IsSensitive,
	}
	notes, err := s.db.SearchSimilar(ctx, vec, topK, filters)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	notes = filterByDistance(notes, s.thresholdFor(opts))
	if len(notes) == 0 {
		return nil, nil
	}

	decrypted, err := s.decryptNotes(notes)
	if err != nil {
		return nil, fmt.Errorf("decrypt notes: %w", err)
	}
	return decrypted, nil
}

// Search embeds the query, retrieves top-K notes, decrypts them,
// and returns an LLM-generated answer alongside the source notes.
func (s *SearchService) Search(ctx context.Context, query string, opts SearchOptions) (*SearchResult, error) {
	decrypted, err := s.Retrieve(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	if len(decrypted) == 0 {
		return &SearchResult{Answer: "No relevant notes found.", Notes: nil}, nil
	}

	step := opts.OnStep
	if step == nil {
		step = func(string) {}
	}

	// LLM generates an answer and tells us which notes it actually used.
	step("thinking")
	answer, err := s.llm.Answer(ctx, query, decrypted)
	if err != nil {
		return nil, fmt.Errorf("llm answer: %w", err)
	}

	// Filter the source list down to notes the LLM said it relied on.
	// When the LLM rejected every candidate (Matched=false), we hide the
	// "Sources" list entirely — the answer alone is shown.
	sources := filterByMatchedIDs(decrypted, answer.UsedNoteIDs)

	return &SearchResult{
		Answer: answer.Answer,
		Notes:  sources,
	}, nil
}

// filterByMatchedIDs returns only the notes whose ID is matched by one of
// the (typically short-prefix) IDs the LLM reported using. An ID matches a
// note if the note's full ID has the LLM-supplied ID as a prefix —
// accommodating both full-UUID and 8-char-short-ID outputs from the model.
func filterByMatchedIDs(notes []*model.Note, matchedIDs []string) []*model.Note {
	if len(matchedIDs) == 0 {
		return nil
	}
	out := make([]*model.Note, 0, len(matchedIDs))
	for _, n := range notes {
		for _, mid := range matchedIDs {
			if mid != "" && strings.HasPrefix(n.ID, mid) {
				out = append(out, n)
				break
			}
		}
	}
	return out
}

// thresholdFor resolves the effective max-distance: caller override wins,
// otherwise the service default. A negative value disables filtering.
func (s *SearchService) thresholdFor(opts SearchOptions) float32 {
	if opts.MaxDistance != 0 {
		return opts.MaxDistance
	}
	return s.maxDistance
}

// filterByDistance returns a new slice containing only notes whose Distance
// is within threshold. A negative threshold disables filtering — the input
// slice is returned as-is. The input is never mutated.
func filterByDistance(notes []*model.Note, threshold float32) []*model.Note {
	if threshold < 0 {
		return notes
	}
	out := make([]*model.Note, 0, len(notes))
	for _, n := range notes {
		if n.Distance <= threshold {
			out = append(out, n)
		}
	}
	return out
}

// decryptNotes returns copies of notes with ContentPlain populated.
// Sensitive notes are decrypted in memory; plaintext is never written to disk.
func (s *SearchService) decryptNotes(notes []*model.Note) ([]*model.Note, error) {
	// Only derive the key once if any note is sensitive.
	var key []byte
	for _, n := range notes {
		if n.IsSensitive && len(n.ContentEnc) > 0 {
			if key == nil {
				var err error
				key, err = s.keyMgr.DeriveKey()
				if err != nil {
					return nil, err
				}
			}
			break
		}
	}

	out := make([]*model.Note, len(notes))
	for i, n := range notes {
		copy := *n // shallow copy — safe, we only modify ContentPlain
		if n.IsSensitive && len(n.ContentEnc) > 0 && key != nil {
			plain, err := Decrypt(key, n.ContentEnc, []byte("note"))
			if err != nil {
				return nil, fmt.Errorf("decrypt note %s: %w", n.ID, err)
			}
			copy.ContentPlain = string(plain)
		}
		out[i] = &copy
	}
	return out, nil
}

// MaskSensitive replaces sensitive values in a string with asterisks.
// Used by the CLI to mask content before display when --reveal is not set.
func MaskSensitive(notes []*model.Note) []*model.Note {
	out := make([]*model.Note, len(notes))
	for i, n := range notes {
		copy := *n
		if n.IsSensitive {
			copy.ContentPlain = maskValue(n.ContentPlain)
		}
		out[i] = &copy
	}
	return out
}

func maskValue(s string) string {
	if len(s) == 0 {
		return "••••••"
	}
	if len(s) <= 8 {
		return "••••••"
	}
	return s[:4] + "••••••"
}
