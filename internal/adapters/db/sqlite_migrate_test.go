package db

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

// withSQLiteMigrations appends test-only migrations for the duration of a test.
func withSQLiteMigrations(t *testing.T, extra ...migration) {
	t.Helper()
	orig := sqliteMigrations
	sqliteMigrations = func(dimension int) []migration {
		return append(orig(dimension), extra...)
	}
	t.Cleanup(func() { sqliteMigrations = orig })
}

func sqliteUserVersion(t *testing.T, a *SQLiteAdapter) int {
	t.Helper()
	var v int
	if err := a.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	return v
}

func backups(t *testing.T, path string) []string {
	t.Helper()
	m, err := filepath.Glob(path + ".bak-*")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// createLegacySQLite builds a database exactly as v0.4.0 left it: the same
// tables, user_version 0, and one note with an embedding.
func createLegacySQLite(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stmts := []string{
		`CREATE TABLE notes (
			id TEXT PRIMARY KEY, content_enc BLOB, content_plain TEXT,
			summary TEXT NOT NULL DEFAULT '', category TEXT NOT NULL DEFAULT 'notes',
			tags TEXT NOT NULL DEFAULT '[]', is_sensitive INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE VIRTUAL TABLE note_embeddings USING vec0(note_id TEXT PRIMARY KEY, embedding FLOAT[4])`,
		`INSERT INTO notes (id, content_plain, summary, category, tags, is_sensitive, created_at, updated_at)
		 VALUES ('legacy-0001', 'docker system prune -a', 'prune docker', 'commands', '["docker"]', 0,
		         '2026-06-01T10:00:00Z', '2026-06-01T10:00:00Z')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("legacy schema: %v", err)
		}
	}
	vec, err := sqlite_vec.SerializeFloat32([]float32{1, 0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO note_embeddings (note_id, embedding) VALUES ('legacy-0001', ?)`, vec); err != nil {
		t.Fatalf("legacy embedding: %v", err)
	}
}

func TestSQLiteMigrate_FreshDatabaseIsLatestWithoutBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.db")
	a, err := NewSQLiteAdapter(path, 4)
	if err != nil {
		t.Fatalf("NewSQLiteAdapter: %v", err)
	}
	defer func() { _ = a.Close() }()

	want := len(sqliteMigrations(4))
	if got := sqliteUserVersion(t, a); got != want {
		t.Errorf("user_version = %d, want %d", got, want)
	}
	if b := backups(t, path); len(b) != 0 {
		t.Errorf("fresh database should not be backed up, found %v", b)
	}
}

func TestSQLiteMigrate_LegacyDatabaseUpgradesWithBackupAndKeepsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.db")
	createLegacySQLite(t, path)

	a, err := NewSQLiteAdapter(path, 4)
	if err != nil {
		t.Fatalf("NewSQLiteAdapter on legacy db: %v", err)
	}
	ctx := context.Background()

	if got, want := sqliteUserVersion(t, a), len(sqliteMigrations(4)); got != want {
		t.Errorf("user_version = %d, want %d", got, want)
	}
	n, err := a.Get(ctx, "legacy-0001")
	if err != nil {
		t.Fatalf("legacy note lost: %v", err)
	}
	if n.ContentPlain != "docker system prune -a" || n.Category != "commands" {
		t.Errorf("legacy note changed: %+v", n)
	}
	hits, err := a.SearchSimilar(ctx, []float32{1, 0, 0, 0}, 1, Filters{})
	if err != nil || len(hits) != 1 || hits[0].ID != "legacy-0001" {
		t.Fatalf("legacy embedding not searchable: hits=%v err=%v", hits, err)
	}

	b := backups(t, path)
	if len(b) != 1 {
		t.Fatalf("want exactly one backup, got %v", b)
	}
	_ = a.Close()

	// The backup is a working copy of the pre-migration database.
	bak, err := sql.Open("sqlite3", b[0])
	if err != nil {
		t.Fatal(err)
	}
	defer bak.Close()
	var count, version int
	if err := bak.QueryRow(`SELECT count(*) FROM notes`).Scan(&count); err != nil || count != 1 {
		t.Errorf("backup notes count = %d, err %v", count, err)
	}
	if err := bak.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 0 {
		t.Errorf("backup user_version = %d, want 0 (err %v)", version, err)
	}
}

func TestSQLiteMigrate_ReopenIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.db")
	createLegacySQLite(t, path)
	for i := 0; i < 2; i++ {
		a, err := NewSQLiteAdapter(path, 4)
		if err != nil {
			t.Fatalf("open #%d: %v", i+1, err)
		}
		_ = a.Close()
	}
	if b := backups(t, path); len(b) != 1 {
		t.Errorf("reopening an up-to-date database must not back it up again, got %v", b)
	}
}

func TestSQLiteMigrate_NewMigrationAppliesOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.db")
	a, err := NewSQLiteAdapter(path, 4)
	if err != nil {
		t.Fatal(err)
	}
	_ = a.Close()

	base := len(sqliteMigrations(4))
	withSQLiteMigrations(t, migration{
		version: base + 1,
		name:    "test column",
		stmts:   []string{`ALTER TABLE notes ADD COLUMN test_col TEXT`},
	})

	for i := 0; i < 2; i++ { // a second open must not re-run ALTER TABLE
		a, err := NewSQLiteAdapter(path, 4)
		if err != nil {
			t.Fatalf("open #%d: %v", i+1, err)
		}
		if got := sqliteUserVersion(t, a); got != base+1 {
			t.Errorf("user_version = %d, want %d", got, base+1)
		}
		_ = a.Close()
	}
}

func TestSQLiteMigrate_FailedMigrationRollsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.db")
	a, err := NewSQLiteAdapter(path, 4)
	if err != nil {
		t.Fatal(err)
	}
	_ = a.Close()

	base := len(sqliteMigrations(4))
	withSQLiteMigrations(t, migration{
		version: base + 1,
		name:    "half applied",
		stmts: []string{
			`ALTER TABLE notes ADD COLUMN good_col TEXT`,
			`THIS IS NOT SQL`,
		},
	})

	if _, err := NewSQLiteAdapter(path, 4); err == nil {
		t.Fatal("expected migration error")
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version, cols int
	_ = db.QueryRow(`PRAGMA user_version`).Scan(&version)
	_ = db.QueryRow(`SELECT count(*) FROM pragma_table_info('notes') WHERE name = 'good_col'`).Scan(&cols)
	if version != base {
		t.Errorf("user_version = %d after failed migration, want %d", version, base)
	}
	if cols != 0 {
		t.Error("first statement of the failed migration was not rolled back")
	}
}

func TestSQLiteMigrate_RefusesNewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 99`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	_, err = NewSQLiteAdapter(path, 4)
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("err = %v, want ErrSchemaTooNew", err)
	}
}

func TestCheckMigrations_RejectsGapsAndEmpty(t *testing.T) {
	if err := checkMigrations(nil); err == nil {
		t.Error("empty list accepted")
	}
	if err := checkMigrations([]migration{{version: 1}, {version: 3}}); err == nil {
		t.Error("gap accepted")
	}
	if err := checkMigrations([]migration{{version: 1}, {version: 2}}); err != nil {
		t.Errorf("valid list rejected: %v", err)
	}
}

func TestSQLiteMigrate_V2MarksOnlySensitiveLegacyNotesUnchecked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.db")
	createLegacySQLite(t, path) // one non-sensitive note
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO notes (id, content_plain, summary, category, tags, is_sensitive, created_at, updated_at)
		VALUES ('legacy-secret', '', 'leaky', 'credentials', '[]', 1, '2026-06-01T10:00:00Z', '2026-06-01T10:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()

	a, err := NewSQLiteAdapter(path, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	ctx := context.Background()

	got, err := a.UncheckedSensitive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "legacy-secret" {
		t.Fatalf("UncheckedSensitive = %v, want only legacy-secret", got)
	}

	if err := a.SetSummary(ctx, "legacy-secret", "credentials (sensitive)"); err != nil {
		t.Fatal(err)
	}
	if left, _ := a.UncheckedSensitive(ctx); len(left) != 0 {
		t.Errorf("still unchecked after SetSummary: %v", left)
	}
	n, err := a.Get(ctx, "legacy-secret")
	if err != nil || n.Summary != "credentials (sensitive)" {
		t.Errorf("summary = %q, err %v", n.Summary, err)
	}
	if err := a.SetSummary(ctx, "does-not-exist", "x"); err == nil {
		t.Error("SetSummary on a missing note should fail")
	}
}
