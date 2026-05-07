package core

import (
	"context"
	"fmt"

	"github.com/shahid-io/inode/internal/adapters/db"
	"github.com/shahid-io/inode/internal/adapters/embedding"
	"github.com/shahid-io/inode/internal/adapters/llm"
	"github.com/shahid-io/inode/internal/model"
)

const defaultTopK = 5

// SearchResult holds both the LLM-generated answer and the raw notes used as context.
type SearchResult struct {
	Answer string
	Notes  []*model.Note
}

// SearchService implements the RAG pipeline for natural language queries.
//
// Flow:
//  1. Embed the user query (Voyage AI)
//  2. Vector similarity search → top-K notes (SQLite/sqlite-vec)
//  3. Decrypt sensitive notes in memory
//  4. Inject notes as context → Claude generates answer
type SearchService struct {
	db        db.Adapter
	embedding embedding.Adapter
	llm       llm.Adapter
	keyMgr    *KeyManager
}

// NewSearchService creates a SearchService with all required dependencies.
func NewSearchService(dbAdapter db.Adapter, embAdapter embedding.Adapter, llmAdapter llm.Adapter, keyMgr *KeyManager) *SearchService {
	return &SearchService{
		db:        dbAdapter,
		embedding: embAdapter,
		llm:       llmAdapter,
		keyMgr:    keyMgr,
	}
}

// SearchOptions carries optional filters for a search query.
type SearchOptions struct {
	TopK     int
	Category string
	Tags     []string
}

// Search embeds the query, retrieves top-K notes, decrypts them,
// and returns an LLM-generated answer alongside the source notes.
func (s *SearchService) Search(ctx context.Context, query string, opts SearchOptions) (*SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	topK := opts.TopK
	if topK <= 0 {
		topK = defaultTopK
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
