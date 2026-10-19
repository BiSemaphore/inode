package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/BiSemaphore/inode/internal/adapters/db"
	"github.com/BiSemaphore/inode/internal/config"
)

// EmbeddingMismatchError means the configured embedding model is not the one
// that made the stored vectors. Searching would compare vectors from
// different models and return unrelated notes without any error, and saving
// would mix the two, so both refuse with this error (#130).
type EmbeddingMismatchError struct {
	Stored     db.EmbeddingSpace
	Configured db.EmbeddingSpace
}

func (e *EmbeddingMismatchError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "the notes in this database were embedded with %s, but the config says %s; "+
		"search would compare vectors from different models.\n", describeSpace(e.Stored), describeSpace(e.Configured))
	if e.Stored.Model == "" {
		fmt.Fprintf(&sb, "Set embedding.backend and embedding.model back to the model you used before (%d dimensions).\n", e.Stored.Dimension)
	} else {
		sb.WriteString("To use these notes, set the embedding settings back:\n")
		fmt.Fprintf(&sb, "  inode config set embedding.backend %s\n", e.Stored.Backend)
		fmt.Fprintf(&sb, "  inode config set embedding.model %s\n", e.Stored.Model)
		if e.Stored.Dimension != e.Configured.Dimension {
			fmt.Fprintf(&sb, "  inode config set embedding.dimension %d\n", e.Stored.Dimension)
		}
	}
	sb.WriteString("Re-embedding stored notes with a new model is not supported yet: https://github.com/BiSemaphore/inode/issues/131")
	return sb.String()
}

func describeSpace(s db.EmbeddingSpace) string {
	if s.Model == "" {
		return fmt.Sprintf("an unknown model (%d dimensions)", s.Dimension)
	}
	return fmt.Sprintf("%s/%s (%d dimensions)", s.Backend, s.Model, s.Dimension)
}

// configuredSpace is the embedding space cfg asks for. cfg must have been
// through ResolveBackends. Ollama treats "name" and "name:latest" as the same
// model, so the suffix is dropped.
func configuredSpace(cfg *config.Config) db.EmbeddingSpace {
	backend, model := cfg.Embedding.Backend, cfg.Embedding.Model
	if backend == "" {
		backend = "ollama"
	}
	if backend == "ollama" {
		model = strings.TrimSuffix(model, ":latest")
	}
	return db.EmbeddingSpace{Backend: backend, Model: model, Dimension: cfg.Embedding.Dimension}
}

// checkEmbeddingSpace compares want with the space recorded in the database
// and returns *EmbeddingMismatchError when they differ. Any other error means
// the check itself failed.
//
// A database without a record (created before v0.6.0) gets one: the vector
// size is read from its schema, and the model is assumed to be the configured
// one when the sizes agree. That is the only information there is; a model
// changed before upgrading, with the same vector size, cannot be detected.
//
// A database without notes adopts want, resizing its vector table if needed,
// since there is nothing to compare against.
func checkEmbeddingSpace(ctx context.Context, store db.Adapter, want db.EmbeddingSpace) error {
	stored, ok, err := store.EmbeddingSpace(ctx)
	if err != nil {
		return err
	}
	if !ok {
		dim, err := store.VectorDimension(ctx)
		if err != nil {
			return err
		}
		stored = db.EmbeddingSpace{Dimension: dim}
		if dim == want.Dimension {
			stored = want
		}
		if err := store.SetEmbeddingSpace(ctx, stored); err != nil {
			return err
		}
	}
	if stored == want {
		return nil
	}

	n, err := store.CountNotes(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return &EmbeddingMismatchError{Stored: stored, Configured: want}
	}
	if stored.Dimension != want.Dimension {
		if err := store.ResizeVectors(ctx, want.Dimension); err != nil {
			return err
		}
	}
	return store.SetEmbeddingSpace(ctx, want)
}

// guardEmbeddingSpace runs checkEmbeddingSpace. On a mismatch, notes.Add and
// search.Retrieve return the mismatch error, while list, note get and note
// delete keep working since they do not use embeddings. Any other error is
// returned.
func guardEmbeddingSpace(ctx context.Context, store db.Adapter, want db.EmbeddingSpace, notes *NoteService, search *SearchService) error {
	err := checkEmbeddingSpace(ctx, store, want)
	var mismatch *EmbeddingMismatchError
	if errors.As(err, &mismatch) {
		notes.embeddingErr = mismatch
		search.embeddingErr = mismatch
		return nil
	}
	return err
}
