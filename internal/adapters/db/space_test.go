package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BiSemaphore/inode/internal/model"
)

// The embedding space tests run the same checks on both backends through the
// Adapter interface. Both adapters start with 4-dimension vectors.

func checkSpaceRoundTrip(t *testing.T, a Adapter) {
	t.Helper()
	ctx := context.Background()

	if _, ok, err := a.EmbeddingSpace(ctx); err != nil || ok {
		t.Fatalf("EmbeddingSpace on a new database = ok %v, err %v; want nothing recorded", ok, err)
	}
	for _, want := range []EmbeddingSpace{
		{Backend: "ollama", Model: "nomic-embed-text", Dimension: 4},
		{Backend: "voyage", Model: "voyage-3", Dimension: 8}, // overwrites the single row
	} {
		if err := a.SetEmbeddingSpace(ctx, want); err != nil {
			t.Fatalf("SetEmbeddingSpace: %v", err)
		}
		got, ok, err := a.EmbeddingSpace(ctx)
		if err != nil || !ok || got != want {
			t.Errorf("EmbeddingSpace = %+v, %v, %v; want %+v", got, ok, err, want)
		}
	}
}

func checkResizeVectors(t *testing.T, a Adapter) {
	t.Helper()
	ctx := context.Background()

	if got, err := a.VectorDimension(ctx); err != nil || got != 4 {
		t.Fatalf("VectorDimension = %d, %v; want 4", got, err)
	}
	if err := a.ResizeVectors(ctx, 8); err != nil {
		t.Fatalf("ResizeVectors on an empty database: %v", err)
	}
	if got, err := a.VectorDimension(ctx); err != nil || got != 8 {
		t.Fatalf("VectorDimension after resize = %d, %v; want 8", got, err)
	}

	vec := []float32{1, 0, 0, 0, 0, 0, 0, 0}
	id, err := a.Save(ctx, &model.Note{ContentPlain: "x", Category: "notes", Embedding: vec})
	if err != nil {
		t.Fatalf("Save with an 8-dimension vector after resize: %v", err)
	}
	hits, err := a.SearchSimilar(ctx, vec, 1, Filters{})
	if err != nil || len(hits) != 1 || hits[0].ID != id {
		t.Fatalf("SearchSimilar after resize = %v, %v", hits, err)
	}
	if n, err := a.CountNotes(ctx); err != nil || n != 1 {
		t.Errorf("CountNotes = %d, %v; want 1", n, err)
	}

	err = a.ResizeVectors(ctx, 4)
	if err == nil || !strings.Contains(err.Error(), "1 notes have embeddings") {
		t.Errorf("ResizeVectors with a stored embedding: err = %v, want refusal", err)
	}
	if got, _ := a.VectorDimension(ctx); got != 8 {
		t.Errorf("VectorDimension after refused resize = %d, want 8", got)
	}
}

func TestSQLite_EmbeddingSpace(t *testing.T) {
	checkSpaceRoundTrip(t, newTestAdapter(t))
}

func TestSQLite_ResizeVectors(t *testing.T) {
	checkResizeVectors(t, newTestAdapter(t))
}

func TestPostgres_EmbeddingSpace(t *testing.T) {
	checkSpaceRoundTrip(t, newPostgresTestAdapter(t))
}

func TestPostgres_ResizeVectors(t *testing.T) {
	a := newPostgresTestAdapter(t)
	// The notes table is shared by the Postgres tests, which expect 4.
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = a.pool.Exec(ctx, "TRUNCATE notes")
		if err := a.ResizeVectors(ctx, 4); err != nil {
			t.Errorf("restore vector size: %v", err)
		}
	})
	checkResizeVectors(t, a)
}

func TestSQLiteMigrate_LegacyDatabaseHasNoEmbeddingSpace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.db")
	createLegacySQLite(t, path)
	a, err := NewSQLiteAdapter(path, 768) // config dimension differs from the schema's 4
	if err != nil {
		t.Fatalf("NewSQLiteAdapter on legacy db: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	ctx := context.Background()

	if _, ok, err := a.EmbeddingSpace(ctx); err != nil || ok {
		t.Errorf("EmbeddingSpace after upgrade = ok %v, err %v; want nothing recorded", ok, err)
	}
	if got, err := a.VectorDimension(ctx); err != nil || got != 4 {
		t.Errorf("VectorDimension = %d, %v; want 4 from the schema, not the 768 passed in", got, err)
	}
}
