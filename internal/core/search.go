package core

import (
	"context"
	"fmt"

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
}

// Search embeds the query, retrieves top-K notes, decrypts them,
// and returns an LLM-generated answer alongside the source notes.
func (s *SearchService) Search(ctx context.Context, query string, opts SearchOptions) (*SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	topK := opts.TopK
	if topK <= 0 {
		topK = s.topK
	}

	// Step 1: embed the query.
	vec, err := s.embedding.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	// Step 2: vector similarity search.
	filters := db.Filters{
		Category: opts.Category,
		Tags:     opts.Tags,
	}
	notes, err := s.db.SearchSimilar(ctx, vec, topK, filters)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	// Step 2b: drop notes beyond the relevance threshold so weak matches
	// never reach the LLM (which would otherwise answer "no match" while
	// the CLI still printed them as Sources).
	notes = filterByDistance(notes, s.thresholdFor(opts))

	if len(notes) == 0 {
		return &SearchResult{Answer: "No relevant notes found.", Notes: nil}, nil
	}

	// Step 3: decrypt sensitive notes in memory.
	decrypted, err := s.decryptNotes(notes)
	if err != nil {
		return nil, fmt.Errorf("decrypt notes: %w", err)
	}

	// Step 4: LLM generates answer from note context.
	answer, err := s.llm.Answer(ctx, query, decrypted)
	if err != nil {
		return nil, fmt.Errorf("llm answer: %w", err)
	}

	return &SearchResult{
		Answer: answer,
		Notes:  decrypted,
	}, nil
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
