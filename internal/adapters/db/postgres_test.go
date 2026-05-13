package db

import (
	"context"
	"os"
	"testing"

	"github.com/shahid-io/inode/internal/model"
)

// newPostgresTestAdapter connects to a real Postgres+pgvector instance via
// INODE_TEST_PG_DSN. When the env var is unset, the whole suite is skipped
// so CI stays green without docker.
//
// Local one-liner to enable these tests:
//
//	docker run -d --name pgvector-test -p 5432:5432 \
//	    -e POSTGRES_PASSWORD=password pgvector/pgvector:pg16
//	INODE_TEST_PG_DSN=postgres://postgres:password@localhost:5432/postgres?sslmode=disable \
//	    go test ./internal/adapters/db/...
//
// Each test truncates the shared `notes` table before running, so tests must
// not be parallelised within this package.
func newPostgresTestAdapter(t *testing.T) *PostgresAdapter {
	t.Helper()
	dsn := os.Getenv("INODE_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("INODE_TEST_PG_DSN not set — skipping Postgres adapter tests")
	}
	ctx := context.Background()
	a, err := NewPostgresAdapter(ctx, dsn, 4)
	if err != nil {
		t.Fatalf("NewPostgresAdapter: %v", err)
	}
	if _, err := a.pool.Exec(ctx, "TRUNCATE notes"); err != nil {
		t.Fatalf("truncate notes: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func TestPostgres_SaveAndGet_FullID(t *testing.T) {
	a := newPostgresTestAdapter(t)
	ctx := context.Background()

	id, err := a.Save(ctx, &model.Note{
		ContentPlain: "echo hello",
		Summary:      "echo hello command",
		Category:     "commands",
		Tags:         []string{"bash"},
		Embedding:    []float32{0.1, 0.2, 0.3, 0.4},
	})
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

func TestPostgres_GetByShortPrefix(t *testing.T) {
	a := newPostgresTestAdapter(t)
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

func TestPostgres_DeleteByShortPrefix(t *testing.T) {
	a := newPostgresTestAdapter(t)
	ctx := context.Background()

	id, _ := a.Save(ctx, &model.Note{Summary: "x", Category: "notes", Embedding: []float32{1, 0, 0, 0}})

	if err := a.Delete(ctx, id[:8]); err != nil {
		t.Fatalf("Delete short prefix: %v", err)
	}

	if _, err := a.Get(ctx, id); err == nil {
		t.Fatal("expected Get to fail after delete")
	}
}

func TestPostgres_SearchSimilar_ReturnsClosestWithDistance(t *testing.T) {
	a := newPostgresTestAdapter(t)
	ctx := context.Background()

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
	if results[0].Distance > results[1].Distance || results[1].Distance > results[2].Distance {
		t.Errorf("distances not ascending: %v %v %v",
			results[0].Distance, results[1].Distance, results[2].Distance)
	}
	if results[0].Distance != 0 {
		t.Errorf("identical vector should have distance 0, got %v", results[0].Distance)
	}
}

func TestPostgres_SearchSimilar_TagFilterAppliedInMemory(t *testing.T) {
	a := newPostgresTestAdapter(t)
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

func TestPostgres_List_FilterByCategory(t *testing.T) {
	a := newPostgresTestAdapter(t)
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

func TestPostgres_Save_StoresEncryptedContent(t *testing.T) {
	a := newPostgresTestAdapter(t)
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

func TestPostgres_Get_NotFound(t *testing.T) {
	a := newPostgresTestAdapter(t)
	ctx := context.Background()

	if _, err := a.Get(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error for missing ID")
	}
}
