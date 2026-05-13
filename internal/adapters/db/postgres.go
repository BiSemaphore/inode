package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
	"github.com/shahid-io/inode/internal/model"
)

// PostgresAdapter implements db.Adapter using PostgreSQL + pgvector.
//
// Compared to the SQLite path, this backend stores the embedding inline on
// the notes table (pgvector's vector(N) column type) rather than in a
// separate virtual table. L2 distance is computed by the `<->` operator,
// matching sqlite-vec's metric so the relevance threshold is portable.
//
// id is stored as TEXT (not UUID) so prefix-match semantics on Get/Delete
// stay identical to the SQLite implementation.
type PostgresAdapter struct {
	pool      *pgxpool.Pool
	dimension int
}

// NewPostgresAdapter connects to Postgres, registers pgvector types, and
// runs migrations. The DSN follows libpq format, e.g.:
//
//	postgres://user:pass@host:5432/db?sslmode=disable
//
// The pgvector extension is created if missing — the connecting role
// therefore needs CREATE privilege on the database (superuser in the
// standard pgvector/pgvector Docker image).
func NewPostgresAdapter(ctx context.Context, dsn string, dimension int) (*PostgresAdapter, error) {
	if dsn == "" {
		return nil, errors.New("postgres dsn is empty — run: inode config set db.dsn <postgres-url>")
	}
	if dimension <= 0 {
		dimension = 768
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvec.RegisterTypes(ctx, conn)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}

	a := &PostgresAdapter{pool: pool, dimension: dimension}
	if err := a.migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return a, nil
}

// migrate enables pgvector and creates the notes table on first run.
//
// Dimension is baked into the schema. If embedding.dimension changes after
// a database is provisioned, the migration is a no-op (CREATE IF NOT EXISTS)
// and inserts of the new size will fail with a clear pgvector error — that
// is the same constraint the SQLite path has, just surfaced differently.
func (a *PostgresAdapter) migrate(ctx context.Context) error {
	if _, err := a.pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		return fmt.Errorf("create extension vector: %w", err)
	}

	stmt := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS notes (
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
		)`, a.dimension)
	if _, err := a.pool.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("create notes table: %w", err)
	}
	return nil
}

// Save persists a note and its embedding. Returns the assigned UUID.
func (a *PostgresAdapter) Save(ctx context.Context, note *model.Note) (string, error) {
	if note.ID == "" {
		note.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	note.CreatedAt = now
	note.UpdatedAt = now

	tagsJSON, err := json.Marshal(note.Tags)
	if err != nil {
		return "", err
	}

	var emb any
	if len(note.Embedding) > 0 {
		emb = pgvector.NewVector(note.Embedding)
	}

	_, err = a.pool.Exec(ctx, `
		INSERT INTO notes (id, content_enc, content_plain, summary, category, tags, is_sensitive, embedding, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		note.ID,
		note.ContentEnc,
		note.ContentPlain,
		note.Summary,
		note.Category,
		string(tagsJSON),
		note.IsSensitive,
		emb,
		now,
		now,
	)
	if err != nil {
		return "", fmt.Errorf("insert note: %w", err)
	}
	return note.ID, nil
}

// Get fetches a single note by ID prefix.
func (a *PostgresAdapter) Get(ctx context.Context, id string) (*model.Note, error) {
	row := a.pool.QueryRow(ctx, `
		SELECT id, content_enc, content_plain, summary, category, tags, is_sensitive, created_at, updated_at
		FROM notes
		WHERE id LIKE $1 || '%'
		LIMIT 1`, id)
	return scanPgNote(row)
}

// Delete removes a note by ID prefix.
func (a *PostgresAdapter) Delete(ctx context.Context, id string) error {
	var fullID string
	err := a.pool.QueryRow(ctx, `SELECT id FROM notes WHERE id LIKE $1 || '%' LIMIT 1`, id).Scan(&fullID)
	if err != nil {
		return fmt.Errorf("note not found: %w", err)
	}
	if _, err := a.pool.Exec(ctx, `DELETE FROM notes WHERE id = $1`, fullID); err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	return nil
}

// SearchSimilar returns top-K notes by L2 distance to vec, ascending.
// Each returned note has Distance populated from the pgvector `<->` operator.
//
// Category filter is pushed down; tag filter is applied in-memory after
// fetching to match the SQLite path's semantics (OR across tags, not AND).
func (a *PostgresAdapter) SearchSimilar(ctx context.Context, vec []float32, topK int, filters Filters) ([]*model.Note, error) {
	args := []any{pgvector.NewVector(vec)}
	query := `
		SELECT embedding <-> $1 AS distance,
		       id, content_enc, content_plain, summary, category, tags, is_sensitive, created_at, updated_at
		FROM notes
		WHERE embedding IS NOT NULL`

	if filters.Category != "" {
		args = append(args, filters.Category)
		query += fmt.Sprintf(` AND category = $%d`, len(args))
	}
	if filters.IsSensitive != nil {
		args = append(args, *filters.IsSensitive)
		query += fmt.Sprintf(` AND is_sensitive = $%d`, len(args))
	}

	args = append(args, topK)
	query += fmt.Sprintf(` ORDER BY distance LIMIT $%d`, len(args))

	rows, err := a.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	defer rows.Close()

	return scanPgScoredNotes(rows, filters)
}

// List returns notes matching filters with pagination.
func (a *PostgresAdapter) List(ctx context.Context, filters Filters, limit, offset int) ([]*model.Note, error) {
	args := []any{}
	query := `
		SELECT id, content_enc, content_plain, summary, category, tags, is_sensitive, created_at, updated_at
		FROM notes
		WHERE 1=1`

	if filters.Category != "" {
		args = append(args, filters.Category)
		query += fmt.Sprintf(` AND category = $%d`, len(args))
	}
	if filters.IsSensitive != nil {
		args = append(args, *filters.IsSensitive)
		query += fmt.Sprintf(` AND is_sensitive = $%d`, len(args))
	}

	args = append(args, limit, offset)
	query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))

	rows, err := a.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPgNotes(rows, filters)
}

// Close releases the pool.
func (a *PostgresAdapter) Close() error {
	a.pool.Close()
	return nil
}

func scanPgNote(row pgx.Row) (*model.Note, error) {
	var n model.Note
	var tagsJSON []byte
	if err := row.Scan(
		&n.ID, &n.ContentEnc, &n.ContentPlain,
		&n.Summary, &n.Category, &tagsJSON,
		&n.IsSensitive, &n.CreatedAt, &n.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(tagsJSON, &n.Tags); err != nil {
		n.Tags = []string{}
	}
	return &n, nil
}

func scanPgNotes(rows pgx.Rows, filters Filters) ([]*model.Note, error) {
	var notes []*model.Note
	for rows.Next() {
		var n model.Note
		var tagsJSON []byte
		if err := rows.Scan(
			&n.ID, &n.ContentEnc, &n.ContentPlain,
			&n.Summary, &n.Category, &tagsJSON,
			&n.IsSensitive, &n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(tagsJSON, &n.Tags); err != nil {
			n.Tags = []string{}
		}
		if matchesTagFilter(&n, filters.Tags) {
			notes = append(notes, &n)
		}
	}
	return notes, rows.Err()
}

func scanPgScoredNotes(rows pgx.Rows, filters Filters) ([]*model.Note, error) {
	var notes []*model.Note
	for rows.Next() {
		var n model.Note
		var distance float64
		var tagsJSON []byte
		if err := rows.Scan(
			&distance,
			&n.ID, &n.ContentEnc, &n.ContentPlain,
			&n.Summary, &n.Category, &tagsJSON,
			&n.IsSensitive, &n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			return nil, err
		}
		n.Distance = float32(distance)
		if err := json.Unmarshal(tagsJSON, &n.Tags); err != nil {
			n.Tags = []string{}
		}
		if matchesTagFilter(&n, filters.Tags) {
			notes = append(notes, &n)
		}
	}
	return notes, rows.Err()
}
