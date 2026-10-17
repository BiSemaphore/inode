package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

// Schema migrations.
//
// Each backend keeps an ordered list of migrations. Versions start at 1 and
// are contiguous. A released migration is never edited: schema changes are
// new migrations appended to the list.
//
// Migration 1 is the schema every database had before versioning existed,
// written with IF NOT EXISTS, so databases created by v0.4.0 and earlier are
// recognised as version 1 without any change to their data.
//
// SQLite records the applied version in PRAGMA user_version and runs each
// migration in its own transaction, after taking a backup of an existing
// database. Postgres records applied versions in a schema_migrations table
// and runs all pending migrations in one transaction under an advisory lock,
// so concurrent processes cannot migrate the same database twice.

type migration struct {
	version int
	name    string
	stmts   []string
}

// ErrSchemaTooNew is returned when a database was migrated by a newer
// inode than the one opening it. Opening it anyway could corrupt data the
// older code does not understand.
var ErrSchemaTooNew = errors.New("database schema is newer than this version of inode supports; upgrade inode")

// pgMigrationLockID is the pg_advisory_xact_lock key used while migrating
// (the ASCII bytes of "inode").
const pgMigrationLockID int64 = 0x696e6f6465

// sqliteMigrations and postgresMigrations are variables so tests can append
// migrations; production code never reassigns them.
var sqliteMigrations = func(dimension int) []migration {
	return []migration{
		{
			version: 1,
			name:    "notes and note_embeddings",
			stmts: []string{
				`CREATE TABLE IF NOT EXISTS notes (
					id            TEXT PRIMARY KEY,
					content_enc   BLOB,
					content_plain TEXT,
					summary       TEXT    NOT NULL DEFAULT '',
					category      TEXT    NOT NULL DEFAULT 'notes',
					tags          TEXT    NOT NULL DEFAULT '[]',
					is_sensitive  INTEGER NOT NULL DEFAULT 1,
					created_at    TEXT    NOT NULL,
					updated_at    TEXT    NOT NULL
				)`,
				fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS note_embeddings USING vec0(
					note_id   TEXT PRIMARY KEY,
					embedding FLOAT[%d]
				)`, dimension),
			},
		},
	}
}

// The embedding dimension is baked into the schema on first run. If
// embedding.dimension changes later, inserts of the new size fail with a
// pgvector error, the same constraint the SQLite path has.
var postgresMigrations = func(dimension int) []migration {
	return []migration{
		{
			version: 1,
			name:    "notes",
			stmts: []string{
				fmt.Sprintf(`CREATE TABLE IF NOT EXISTS notes (
					id            TEXT PRIMARY KEY,
					content_enc   BYTEA,
					content_plain TEXT,
					summary       TEXT        NOT NULL DEFAULT '',
					category      TEXT        NOT NULL DEFAULT 'notes',
					tags          JSONB       NOT NULL DEFAULT '[]'::jsonb,
					is_sensitive  BOOLEAN     NOT NULL DEFAULT TRUE,
					embedding     vector(%d),
					created_at    TIMESTAMPTZ NOT NULL,
					updated_at    TIMESTAMPTZ NOT NULL
				)`, dimension),
			},
		},
	}
}

// checkMigrations guards against editing mistakes in the lists above.
func checkMigrations(ms []migration) error {
	if len(ms) == 0 {
		return errors.New("no migrations defined")
	}
	for i, m := range ms {
		if m.version != i+1 {
			return fmt.Errorf("migration %q has version %d, want %d (versions must be contiguous from 1)", m.name, m.version, i+1)
		}
	}
	return nil
}

// migrate brings a SQLite database to the latest schema version. path is
// the database file; it is used for the pre-migration backup.
func (a *SQLiteAdapter) migrate(path string) error {
	ms := sqliteMigrations(a.dimension)
	if err := checkMigrations(ms); err != nil {
		return err
	}
	latest := ms[len(ms)-1].version

	var current int
	if err := a.db.QueryRow(`PRAGMA user_version`).Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if current > latest {
		return fmt.Errorf("%w (database is at version %d, this build knows up to %d)", ErrSchemaTooNew, current, latest)
	}
	if current == latest {
		return nil
	}

	if err := a.backupBeforeMigrate(path, current); err != nil {
		return err
	}

	for _, m := range ms[current:] {
		if err := a.apply(m); err != nil {
			return fmt.Errorf("migration %d (%s): %w", m.version, m.name, err)
		}
	}
	return nil
}

func (a *SQLiteAdapter) apply(m migration) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, stmt := range m.stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	// PRAGMA does not accept bound parameters; version is an int we control.
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, m.version)); err != nil {
		return err
	}
	return tx.Commit()
}

// backupBeforeMigrate copies an existing database next to itself before
// its schema changes, because for most users this file is the only copy of
// their notes. Fresh databases (no notes table yet) and in-memory databases
// are not backed up. VACUUM INTO writes a consistent copy even in WAL mode.
func (a *SQLiteAdapter) backupBeforeMigrate(path string, fromVersion int) error {
	if fi, err := os.Stat(path); err != nil || !fi.Mode().IsRegular() {
		return nil
	}
	var tables int
	if err := a.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'notes'`).Scan(&tables); err != nil {
		return fmt.Errorf("inspect schema: %w", err)
	}
	if tables == 0 {
		return nil
	}

	backup := fmt.Sprintf("%s.bak-v%d-%s", path, fromVersion, time.Now().UTC().Format("20060102T150405Z"))
	if _, err := a.db.Exec(`VACUUM INTO ?`, backup); err != nil {
		return fmt.Errorf("back up database to %s before migrating: %w", backup, err)
	}
	return nil
}

// migrate brings a Postgres database to the latest schema version.
func (a *PostgresAdapter) migrate(ctx context.Context) error {
	ms := postgresMigrations(a.dimension)
	if err := checkMigrations(ms); err != nil {
		return err
	}
	latest := ms[len(ms)-1].version

	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, pgMigrationLockID); err != nil {
		return fmt.Errorf("lock for migration: %w", err)
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER     PRIMARY KEY,
			name       TEXT        NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var current int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if current > latest {
		return fmt.Errorf("%w (database is at version %d, this build knows up to %d)", ErrSchemaTooNew, current, latest)
	}

	for _, m := range ms[current:] {
		for _, stmt := range m.stmts {
			if _, err := tx.Exec(ctx, stmt); err != nil {
				return fmt.Errorf("migration %d (%s): %w", m.version, m.name, err)
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, m.version, m.name); err != nil {
			return fmt.Errorf("record migration %d: %w", m.version, err)
		}
	}
	return tx.Commit(ctx)
}
