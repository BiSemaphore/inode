package db

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/BiSemaphore/inode/internal/model"
)

// Postgres migration tests run against INODE_TEST_PG_DSN like the other
// Postgres tests, and are skipped without it.

func pgSchemaVersion(t *testing.T, a *PostgresAdapter) int {
	t.Helper()
	var v int
	if err := a.pool.QueryRow(context.Background(), `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&v); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	return v
}

func withPostgresMigrations(t *testing.T, extra ...migration) {
	t.Helper()
	orig := postgresMigrations
	postgresMigrations = func(dimension int) []migration {
		return append(orig(dimension), extra...)
	}
	t.Cleanup(func() { postgresMigrations = orig })
}

func TestPostgresMigrate_RecordsLatestVersion(t *testing.T) {
	a := newPostgresTestAdapter(t)
	if got, want := pgSchemaVersion(t, a), len(postgresMigrations(4)); got != want {
		t.Errorf("schema version = %d, want %d", got, want)
	}
}

func TestPostgresMigrate_FailedMigrationRollsBackEverything(t *testing.T) {
	a := newPostgresTestAdapter(t)
	ctx := context.Background()
	base := pgSchemaVersion(t, a)

	withPostgresMigrations(t,
		migration{version: base + 1, name: "ok", stmts: []string{`ALTER TABLE notes ADD COLUMN pg_test_col TEXT`}},
		migration{version: base + 2, name: "broken", stmts: []string{`THIS IS NOT SQL`}},
	)
	if _, err := NewPostgresAdapter(ctx, os.Getenv("INODE_TEST_PG_DSN"), 4); err == nil {
		t.Fatal("expected migration error")
	}

	if got := pgSchemaVersion(t, a); got != base {
		t.Errorf("schema version = %d after failed run, want %d", got, base)
	}
	var cols int
	if err := a.pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_name = 'notes' AND column_name = 'pg_test_col'`).Scan(&cols); err != nil {
		t.Fatal(err)
	}
	if cols != 0 {
		t.Error("earlier migration in the same run was not rolled back")
	}
}

func TestPostgresMigrate_RefusesNewerSchema(t *testing.T) {
	a := newPostgresTestAdapter(t)
	ctx := context.Background()
	if _, err := a.pool.Exec(ctx, `INSERT INTO schema_migrations (version, name) VALUES (99, 'from the future')`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = a.pool.Exec(ctx, `DELETE FROM schema_migrations WHERE version = 99`) })

	_, err := NewPostgresAdapter(ctx, os.Getenv("INODE_TEST_PG_DSN"), 4)
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("err = %v, want ErrSchemaTooNew", err)
	}
}

func TestPostgres_UncheckedSensitiveAndSetSummary(t *testing.T) {
	a := newPostgresTestAdapter(t)
	ctx := context.Background()

	// Simulate a note saved before summaries were validated.
	if _, err := a.pool.Exec(ctx, `INSERT INTO notes (id, content_plain, summary, category, tags, is_sensitive, created_at, updated_at, summary_checked)
		VALUES ('pg-legacy', '', 'leaky', 'credentials', '[]', TRUE, now(), now(), FALSE)`); err != nil {
		t.Fatal(err)
	}
	// A note saved through the adapter is born checked.
	if _, err := a.Save(ctx, &model.Note{Summary: "fine", Category: "notes", IsSensitive: true, Embedding: []float32{1, 0, 0, 0}}); err != nil {
		t.Fatal(err)
	}

	got, err := a.UncheckedSensitive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "pg-legacy" {
		t.Fatalf("UncheckedSensitive = %v, want only pg-legacy", got)
	}
	if err := a.SetSummary(ctx, "pg-legacy", "credentials (sensitive)"); err != nil {
		t.Fatal(err)
	}
	if left, _ := a.UncheckedSensitive(ctx); len(left) != 0 {
		t.Errorf("still unchecked: %v", left)
	}
	if err := a.SetSummary(ctx, "missing", "x"); err == nil {
		t.Error("SetSummary on a missing note should fail")
	}
}
