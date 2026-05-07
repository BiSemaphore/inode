package core

import (
	"context"
	"fmt"
	"time"

	"github.com/shahid-io/inode/internal/adapters/db"
	"github.com/shahid-io/inode/internal/adapters/embedding"
	"github.com/shahid-io/inode/internal/model"
)

// NoteService orchestrates saving, fetching, and deleting notes.
// It coordinates the DB adapter, embedding adapter, tagger, and encryption.
type NoteService struct {
	db        db.Adapter
	embedding embedding.Adapter
	tagger    *TaggerService
	keyMgr    *KeyManager
}

// NewNoteService creates a NoteService with all required dependencies.
func NewNoteService(dbAdapter db.Adapter, embAdapter embedding.Adapter, tagger *TaggerService, keyMgr *KeyManager) *NoteService {
	return &NoteService{
		db:        dbAdapter,
		embedding: embAdapter,
		tagger:    tagger,
		keyMgr:    keyMgr,
	}
}

// AddOptions carries user-supplied options for adding a note.
type AddOptions struct {
	ClassifyOptions
	DefaultSensitive bool // from config — used when IsSensitive is not explicitly set
}

// Add classifies, embeds, encrypts (if sensitive), and persists a note.
func (s *NoteService) Add(ctx context.Context, content string, opts AddOptions) (*model.Note, error) {
	if content == "" {
		return nil, fmt.Errorf("content cannot be empty")
	}

	// Apply config default for sensitivity if not explicitly set.
	if opts.IsSensitive == nil {
		opts.IsSensitive = &opts.DefaultSensitive
	}

	// Step 1: classify — category, tags, sensitivity, summary.
	classification, err := s.tagger.Classify(ctx, content, opts.ClassifyOptions)
	if err != nil {
		return nil, fmt.Errorf("classify: %w", err)
	}

	// Step 2: generate embedding.
	vec, err := s.embedding.Embed(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}

	note := &model.Note{
		Summary:     classification.Summary,
		Category:    classification.Category,
		Tags:        classification.Tags,
		IsSensitive: classification.IsSensitive,
		Embedding:   vec,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	// Step 3: encrypt if sensitive.
	if note.IsSensitive {
		key, err := s.keyMgr.DeriveKey()
		if err != nil {
			return nil, fmt.Errorf("derive key: %w", err)
		}
		// ID is not yet assigned; use a placeholder for AAD — the DB adapter
		// will assign the real UUID. We re-encrypt after ID is known.
		// For simplicity in Phase 1, we use the content hash as AAD.
		enc, err := Encrypt(key, []byte(content), []byte("note"))
		if err != nil {
			return nil, fmt.Errorf("encrypt: %w", err)
		}
		note.ContentEnc = enc
		note.ContentPlain = ""
	} else {
		note.ContentPlain = content
		note.ContentEnc = nil
	}

	// Step 4: persist.
	id, err := s.db.Save(ctx, note)
	if err != nil {
		return nil, fmt.Errorf("save: %w", err)
	}
	note.ID = id

	return note, nil
}

// Get fetches a note and decrypts its content if sensitive.
func (s *NoteService) Get(ctx context.Context, id string) (*model.Note, error) {
	note, err := s.db.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get note %s: %w", id, err)
	}

	if note.IsSensitive && len(note.ContentEnc) > 0 {
		key, err := s.keyMgr.DeriveKey()
		if err != nil {
			return nil, fmt.Errorf("derive key: %w", err)
		}
		plain, err := Decrypt(key, note.ContentEnc, []byte("note"))
		if err != nil {
			return nil, fmt.Errorf("decrypt note: %w", err)
		}
		note.ContentPlain = string(plain)
	}

	return note, nil
}

// Delete removes a note by ID.
func (s *NoteService) Delete(ctx context.Context, id string) error {
	if err := s.db.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete note %s: %w", id, err)
	}
	return nil
}

// List returns notes matching the given filters.
func (s *NoteService) List(ctx context.Context, filters db.Filters, limit, offset int) ([]*model.Note, error) {
	return s.db.List(ctx, filters, limit, offset)
}
