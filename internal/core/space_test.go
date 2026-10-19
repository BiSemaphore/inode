package core

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BiSemaphore/inode/internal/adapters/db"
	"github.com/BiSemaphore/inode/internal/config"
	"github.com/BiSemaphore/inode/internal/model"
)

var (
	spaceA4 = db.EmbeddingSpace{Backend: "ollama", Model: "a", Dimension: 4}
	spaceB4 = db.EmbeddingSpace{Backend: "ollama", Model: "b", Dimension: 4}
	spaceA8 = db.EmbeddingSpace{Backend: "ollama", Model: "a", Dimension: 8}
)

// openSpaceDB opens a 4-dimension SQLite database at path, holding notes
// notes. Reopening the same path simulates the next run of inode.
func openSpaceDB(t *testing.T, path string, notes int) *db.SQLiteAdapter {
	t.Helper()
	store, err := db.NewSQLiteAdapter(path, 4)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for range notes {
		n := &model.Note{ContentPlain: "x", Category: "notes", Embedding: []float32{1, 0, 0, 0}}
		if _, err := store.Save(context.Background(), n); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	return store
}

func recorded(t *testing.T, store db.Adapter) db.EmbeddingSpace {
	t.Helper()
	s, ok, err := store.EmbeddingSpace(context.Background())
	if err != nil || !ok {
		t.Fatalf("EmbeddingSpace = ok %v, err %v", ok, err)
	}
	return s
}

func TestCheckEmbeddingSpace_SameModelPasses(t *testing.T) {
	ctx := context.Background()
	store := openSpaceDB(t, filepath.Join(t.TempDir(), "notes.db"), 0)

	if err := checkEmbeddingSpace(ctx, store, spaceA4); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if got := recorded(t, store); got != spaceA4 {
		t.Errorf("recorded %+v, want %+v", got, spaceA4)
	}
	if _, err := store.Save(ctx, &model.Note{ContentPlain: "x", Category: "notes", Embedding: []float32{1, 0, 0, 0}}); err != nil {
		t.Fatal(err)
	}
	if err := checkEmbeddingSpace(ctx, store, spaceA4); err != nil {
		t.Errorf("second run, same model: %v", err)
	}
}

func TestCheckEmbeddingSpace_MismatchWithNotes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		want      db.EmbeddingSpace
		inMessage []string
		notIn     string
	}{
		{
			name: "other model, same dimension",
			want: spaceB4,
			inMessage: []string{
				"embedded with ollama/a (4 dimensions), but the config says ollama/b (4 dimensions)",
				"inode config set embedding.backend ollama",
				"inode config set embedding.model a",
				"issues/131",
			},
			notIn: "embedding.dimension",
		},
		{
			name: "same model, other dimension",
			want: spaceA8,
			inMessage: []string{
				"but the config says ollama/a (8 dimensions)",
				"inode config set embedding.dimension 4",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			store := openSpaceDB(t, filepath.Join(t.TempDir(), "notes.db"), 1)
			if err := store.SetEmbeddingSpace(ctx, spaceA4); err != nil {
				t.Fatal(err)
			}

			err := checkEmbeddingSpace(ctx, store, tc.want)
			var mismatch *EmbeddingMismatchError
			if !errors.As(err, &mismatch) {
				t.Fatalf("err = %v, want *EmbeddingMismatchError", err)
			}
			for _, s := range tc.inMessage {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("message lacks %q:\n%s", s, err)
				}
			}
			if tc.notIn != "" && strings.Contains(err.Error(), tc.notIn) {
				t.Errorf("message mentions %q:\n%s", tc.notIn, err)
			}
			if got := recorded(t, store); got != spaceA4 {
				t.Errorf("record changed to %+v on a mismatch", got)
			}
		})
	}
}

func TestCheckEmbeddingSpace_EmptyDatabaseAdoptsConfig(t *testing.T) {
	ctx := context.Background()
	store := openSpaceDB(t, filepath.Join(t.TempDir(), "notes.db"), 0)
	if err := store.SetEmbeddingSpace(ctx, spaceA4); err != nil {
		t.Fatal(err)
	}

	if err := checkEmbeddingSpace(ctx, store, spaceA8); err != nil {
		t.Fatalf("empty database, new model: %v", err)
	}
	if got := recorded(t, store); got != spaceA8 {
		t.Errorf("recorded %+v, want %+v", got, spaceA8)
	}
	if got, _ := store.VectorDimension(ctx); got != 8 {
		t.Errorf("VectorDimension = %d, want 8", got)
	}
}

func TestCheckEmbeddingSpace_DatabaseFromBeforeV060(t *testing.T) {
	ctx := context.Background()

	// Same vector size: the configured model is assumed and recorded.
	store := openSpaceDB(t, filepath.Join(t.TempDir(), "notes.db"), 1)
	if err := checkEmbeddingSpace(ctx, store, spaceB4); err != nil {
		t.Fatalf("same dimension: %v", err)
	}
	if got := recorded(t, store); got != spaceB4 {
		t.Errorf("recorded %+v, want %+v", got, spaceB4)
	}

	// Different vector size: the model is unknown, and the size comes from
	// the schema, not from the config.
	store = openSpaceDB(t, filepath.Join(t.TempDir(), "notes.db"), 1)
	err := checkEmbeddingSpace(ctx, store, spaceA8)
	var mismatch *EmbeddingMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("err = %v, want *EmbeddingMismatchError", err)
	}
	if want := (db.EmbeddingSpace{Dimension: 4}); mismatch.Stored != want {
		t.Errorf("Stored = %+v, want %+v", mismatch.Stored, want)
	}
	for _, s := range []string{"an unknown model (4 dimensions)", "back to the model you used before (4 dimensions)"} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("message lacks %q:\n%s", s, err)
		}
	}
}

func TestConfiguredSpace(t *testing.T) {
	for _, tc := range []struct {
		backend, model, wantBackend, wantModel string
	}{
		{"", "nomic-embed-text", "ollama", "nomic-embed-text"},
		{"ollama", "nomic-embed-text:latest", "ollama", "nomic-embed-text"},
		{"ollama", "nomic-embed-text:v1.5", "ollama", "nomic-embed-text:v1.5"},
		{"voyage", "voyage-3", "voyage", "voyage-3"},
	} {
		cfg := &config.Config{}
		cfg.Embedding.Backend, cfg.Embedding.Model, cfg.Embedding.Dimension = tc.backend, tc.model, 768
		got := configuredSpace(cfg)
		want := db.EmbeddingSpace{Backend: tc.wantBackend, Model: tc.wantModel, Dimension: 768}
		if got != want {
			t.Errorf("configuredSpace(%q, %q) = %+v, want %+v", tc.backend, tc.model, got, want)
		}
	}
}

func TestGuardEmbeddingSpace_MismatchBlocksAddAndSearchOnly(t *testing.T) {
	ctx := context.Background()
	store := openSpaceDB(t, filepath.Join(t.TempDir(), "notes.db"), 1)
	if err := store.SetEmbeddingSpace(ctx, spaceA4); err != nil {
		t.Fatal(err)
	}
	// No embedding, LLM or key manager: a call that got past the guard
	// would fail differently or panic.
	notes := NewNoteService(store, nil, nil, nil)
	search := NewSearchService(store, nil, nil, nil, 0, 0)

	if err := guardEmbeddingSpace(ctx, store, spaceB4, notes, search); err != nil {
		t.Fatalf("guardEmbeddingSpace: %v", err)
	}

	var mismatch *EmbeddingMismatchError
	if _, err := notes.Add(ctx, "new note", AddOptions{}); !errors.As(err, &mismatch) {
		t.Errorf("Add: err = %v, want *EmbeddingMismatchError", err)
	}
	if _, err := search.Search(ctx, "query", SearchOptions{}); !errors.As(err, &mismatch) {
		t.Errorf("Search: err = %v, want *EmbeddingMismatchError", err)
	}
	if _, err := search.Retrieve(ctx, "query", SearchOptions{}); !errors.As(err, &mismatch) {
		t.Errorf("Retrieve: err = %v, want *EmbeddingMismatchError", err)
	}
	list, err := notes.List(ctx, db.Filters{}, 10, 0)
	if err != nil || len(list) != 1 {
		t.Errorf("List = %d notes, %v; want 1, nil", len(list), err)
	}
	if _, err := notes.Get(ctx, list[0].ID); err != nil {
		t.Errorf("Get: %v", err)
	}
	if err := notes.Delete(ctx, list[0].ID); err != nil {
		t.Errorf("Delete: %v", err)
	}
}

func TestGuardEmbeddingSpace_MatchLeavesServicesAlone(t *testing.T) {
	ctx := context.Background()
	store := openSpaceDB(t, filepath.Join(t.TempDir(), "notes.db"), 1)
	notes := NewNoteService(store, nil, nil, nil)
	search := NewSearchService(store, nil, nil, nil, 0, 0)

	if err := guardEmbeddingSpace(ctx, store, spaceA4, notes, search); err != nil {
		t.Fatalf("guardEmbeddingSpace: %v", err)
	}
	if notes.embeddingErr != nil || search.embeddingErr != nil {
		t.Errorf("services blocked on a matching model: %v / %v", notes.embeddingErr, search.embeddingErr)
	}
}
