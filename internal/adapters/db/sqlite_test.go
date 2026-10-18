package db

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BiSemaphore/inode/internal/model"
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

func TestSQLite_MatchIDs(t *testing.T) {
	// Regression for #90: Get and Delete used LIKE prefix% LIMIT 1, so an
	// ambiguous prefix picked one note arbitrarily and "" or "%" matched any.
	a := newTestAdapter(t)
	ctx := context.Background()

	for _, id := range []string{"3f1a0000-aaaa", "3b770000-bbbb", "c0de0000-cccc", "a_b%0000-dddd"} {
		if _, err := a.Save(ctx, &model.Note{ID: id, Summary: id, Category: "notes", Embedding: []float32{1, 0, 0, 0}}); err != nil {
			t.Fatalf("Save %s: %v", id, err)
		}
	}

	tests := []struct {
		prefix string
		limit  int
		want   []string
	}{
		{"3", 10, []string{"3b770000-bbbb", "3f1a0000-aaaa"}},
		{"3", 1, []string{"3b770000-bbbb"}},
		{"3f1a", 10, []string{"3f1a0000-aaaa"}},
		{"c0de0000-cccc", 10, []string{"c0de0000-cccc"}},
		{"%", 10, nil},
		{"a%", 10, nil},
		{"a_b%", 10, []string{"a_b%0000-dddd"}},
		{"ffff", 10, nil},
	}
	for _, tt := range tests {
		got, err := a.MatchIDs(ctx, tt.prefix, tt.limit)
		if err != nil {
			t.Fatalf("MatchIDs(%q): %v", tt.prefix, err)
		}
		if strings.Join(got, ",") != strings.Join(tt.want, ",") {
			t.Errorf("MatchIDs(%q, %d) = %v, want %v", tt.prefix, tt.limit, got, tt.want)
		}
	}
}

func TestSQLite_GetAndDeleteNeedFullID(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	id, err := a.Save(ctx, &model.Note{Summary: "x", Category: "notes", Embedding: []float32{1, 0, 0, 0}})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := a.Get(ctx, id[:8]); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(prefix) error = %v, want ErrNotFound", err)
	}
	if err := a.Delete(ctx, id[:8]); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete(prefix) error = %v, want ErrNotFound", err)
	}
	if err := a.Delete(ctx, id); err != nil {
		t.Fatalf("Delete(full ID): %v", err)
	}
	if _, err := a.Get(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after delete error = %v, want ErrNotFound", err)
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
