package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/shahid-io/inode/internal/model"
)

// newTestAdapter spins up a real SQLite DB inside a per-test temp dir.
// Each test gets its own file so they don't share state.
func newTestAdapter(t *testing.T) *SQLiteAdapter {
	t.Helper()
	path := filepath.Join(t.TempDir(), "notes.db")
	a, err := NewSQLiteAdapter(path, 4)
	if err != nil {
		t.Fatalf("NewSQLiteAdapter: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func TestSQLite_SaveAndGet_FullID(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	n := &model.Note{
		ContentPlain: "echo hello",
		Summary:      "echo hello command",
		Category:     "commands",
		Tags:         []string{"bash"},
		IsSensitive:  false,
		Embedding:    []float32{0.1, 0.2, 0.3, 0.4},
	}

	id, err := a.Save(ctx, n)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := a.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get(%q): %v", id, err)
	}
	if got.Summary != "echo hello command" {
		t.Errorf("Summary mismatch: %q", got.Summary)
	}
	if got.Category != "commands" {
		t.Errorf("Category mismatch: %q", got.Category)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "bash" {
		t.Errorf("Tags mismatch: %v", got.Tags)
	}
}

func TestSQLite_GetByShortPrefix(t *testing.T) {
	// Regression for the bug fix that taught Get to accept a short ID prefix.
	a := newTestAdapter(t)
	ctx := context.Background()

	id, err := a.Save(ctx, &model.Note{
		ContentPlain: "x",
		Summary:      "n",
		Category:     "notes",
		Embedding:    []float32{0.1, 0.2, 0.3, 0.4},
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := a.Get(ctx, id[:8])
	if err != nil {
		t.Fatalf("Get short prefix: %v", err)
	}
	if got.ID != id {
		t.Errorf("expected full ID %q, got %q", id, got.ID)
	}
}

func TestSQLite_DeleteByShortPrefix(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	id, _ := a.Save(ctx, &model.Note{Summary: "x", Category: "notes", Embedding: []float32{1, 0, 0, 0}})

	if err := a.Delete(ctx, id[:8]); err != nil {
		t.Fatalf("Delete short prefix: %v", err)
	}

	_, err := a.Get(ctx, id)
	if err == nil {
		t.Fatal("expected Get to fail after delete")
	}
}

func TestSQLite_SearchSimilar_ReturnsClosestWithDistance(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	// Three notes with deliberately spaced embeddings. The query (1,0,0,0)
	// should match note A (1,0,0,0) closest, then C (0.9,0.1,0,0), then B (0,1,0,0).
	mustSave := func(summary string, vec []float32) string {
		id, err := a.Save(ctx, &model.Note{Summary: summary, Category: "notes", Embedding: vec})
		if err != nil {
			t.Fatalf("Save %s: %v", summary, err)
		}
		return id
	}
	idA := mustSave("note A", []float32{1, 0, 0, 0})
	mustSave("note B", []float32{0, 1, 0, 0})
	idC := mustSave("note C", []float32{0.9, 0.1, 0, 0})

	results, err := a.SearchSimilar(ctx, []float32{1, 0, 0, 0}, 3, Filters{})
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].ID != idA {
		t.Errorf("nearest should be note A, got %s", results[0].Summary)
	}
	if results[1].ID != idC {
		t.Errorf("second-nearest should be note C, got %s", results[1].Summary)
	}

	// Distance must be populated and ordered ascending.
	if results[0].Distance > results[1].Distance || results[1].Distance > results[2].Distance {
		t.Errorf("distances not ascending: %v %v %v",
			results[0].Distance, results[1].Distance, results[2].Distance)
	}
	if results[0].Distance != 0 {
		t.Errorf("identical vector should have distance 0, got %v", results[0].Distance)
	}
}

func TestSQLite_SearchSimilar_TagFilterAppliedInMemory(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	mustSave := func(n *model.Note) {
		t.Helper()
		if _, err := a.Save(ctx, n); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}
	mustSave(&model.Note{Summary: "stripe", Category: "credentials", Tags: []string{"stripe", "payment"}, Embedding: []float32{1, 0, 0, 0}})
	mustSave(&model.Note{Summary: "github", Category: "credentials", Tags: []string{"github"}, Embedding: []float32{0.95, 0.05, 0, 0}})
	mustSave(&model.Note{Summary: "aws", Category: "credentials", Tags: []string{"aws"}, Embedding: []float32{0.9, 0.1, 0, 0}})

	results, err := a.SearchSimilar(ctx, []float32{1, 0, 0, 0}, 5, Filters{Tags: []string{"github", "aws"}})
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results matching github|aws filter, got %d", len(results))
	}
	for _, r := range results {
		if r.Summary == "stripe" {
			t.Errorf("stripe note should have been filtered out by tag filter")
		}
	}
}

func TestSQLite_List_FilterByCategory(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	mustSave := func(n *model.Note) {
		t.Helper()
		if _, err := a.Save(ctx, n); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}
	mustSave(&model.Note{Summary: "key1", Category: "credentials", Embedding: []float32{1, 0, 0, 0}})
	mustSave(&model.Note{Summary: "key2", Category: "credentials", Embedding: []float32{0, 1, 0, 0}})
	mustSave(&model.Note{Summary: "ls -la", Category: "commands", Embedding: []float32{0, 0, 1, 0}})

	results, err := a.List(ctx, Filters{Category: "credentials"}, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(results))
	}
	for _, r := range results {
		if r.Category != "credentials" {
			t.Errorf("category leak: %q", r.Category)
		}
	}
}

func TestSQLite_Save_StoresEncryptedContent(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	encrypted := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x01, 0x02}
	id, err := a.Save(ctx, &model.Note{
		ContentEnc:  encrypted,
		Summary:     "secret",
		Category:    "credentials",
		IsSensitive: true,
		Embedding:   []float32{1, 0, 0, 0},
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := a.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.IsSensitive {
		t.Error("expected IsSensitive=true to be persisted")
	}
	if string(got.ContentEnc) != string(encrypted) {
		t.Errorf("ContentEnc mismatch — got %x", got.ContentEnc)
	}
	if got.ContentPlain != "" {
		t.Errorf("ContentPlain should be empty for sensitive note, got %q", got.ContentPlain)
	}
}

func TestSQLite_Get_NotFound(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	_, err := a.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
}
