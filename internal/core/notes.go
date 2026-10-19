package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/BiSemaphore/inode/internal/adapters/db"
	"github.com/BiSemaphore/inode/internal/adapters/embedding"
	"github.com/BiSemaphore/inode/internal/model"
)

// NoteService orchestrates saving, fetching, and deleting notes.
// It coordinates the DB adapter, embedding adapter, tagger, and encryption.
type NoteService struct {
	db        db.Adapter
	embedding embedding.Adapter
	tagger    *TaggerService
	keyMgr    *KeyManager

	// embeddingErr, when set, is returned by Add: the configured embedding
	// model is not the one the stored notes were embedded with.
	embeddingErr error
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
	if s.embeddingErr != nil {
		return nil, s.embeddingErr
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

// maxMatches is how many candidates an ambiguous ID error lists.
const maxMatches = 5

// ErrInvalidID is returned for an empty ID or one with characters that never
// appear in a note ID (IDs are UUIDs: hex digits and dashes).
var ErrInvalidID = errors.New("invalid note ID: use the hex ID shown by inode list")

// AmbiguousIDError is returned when an ID prefix matches more than one note.
// Matches holds up to maxMatches of them; More is true when there are others.
type AmbiguousIDError struct {
	Prefix  string
	Matches []*model.Note
	More    bool
}

func (e *AmbiguousIDError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "note ID %q matches more than one note, use more characters:", e.Prefix)
	for _, n := range e.Matches {
		// The error can reach an MCP client, which never sees sensitive
		// notes unless mcp.reveal_sensitive is set, so leave their summary out.
		if n.IsSensitive {
			fmt.Fprintf(&sb, "\n  #%s  (sensitive) [%s]", ShortID(n.ID), n.Category)
		} else {
			fmt.Fprintf(&sb, "\n  #%s  %s [%s]", ShortID(n.ID), n.Summary, n.Category)
		}
	}
	if e.More {
		sb.WriteString("\n  ...")
	}
	return sb.String()
}

// ShortID is the 8-character form of a note ID shown by list and search.
func ShortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// Resolve finds the one note that ref names: a full ID or a prefix of one.
// It fails with ErrInvalidID, db.ErrNotFound or *AmbiguousIDError rather than
// guessing. Content of a sensitive note is returned still encrypted.
func (s *NoteService) Resolve(ctx context.Context, ref string) (*model.Note, error) {
	prefix := strings.ToLower(strings.TrimSpace(ref))
	if prefix == "" || strings.Trim(prefix, "0123456789abcdef-") != "" {
		return nil, ErrInvalidID
	}

	ids, err := s.db.MatchIDs(ctx, prefix, maxMatches+1)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: %s", db.ErrNotFound, prefix)
	}

	var notes []*model.Note
	for _, id := range ids[:min(len(ids), maxMatches)] {
		n, err := s.db.Get(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get note %s: %w", ShortID(id), err)
		}
		notes = append(notes, n)
	}
	if len(notes) == 1 {
		return notes[0], nil
	}
	return nil, &AmbiguousIDError{Prefix: prefix, Matches: notes, More: len(ids) > maxMatches}
}

// Get resolves ref (see Resolve) and decrypts the note's content if sensitive.
func (s *NoteService) Get(ctx context.Context, ref string) (*model.Note, error) {
	note, err := s.Resolve(ctx, ref)
	if err != nil {
		return nil, err
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

// Delete removes a note by its full ID. Callers resolve user input with
// Resolve first, so a prefix can never delete a note the user did not see.
func (s *NoteService) Delete(ctx context.Context, id string) error {
	if err := s.db.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete note %s: %w", ShortID(id), err)
	}
	return nil
}

// List returns notes matching the given filters.
func (s *NoteService) List(ctx context.Context, filters db.Filters, limit, offset int) ([]*model.Note, error) {
	return s.db.List(ctx, filters, limit, offset)
}
