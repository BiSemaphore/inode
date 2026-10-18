package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/BiSemaphore/inode/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
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
	// Ensure the pgvector extension exists before registering its types —
	// pgxvec.RegisterTypes looks the `vector` type up in pg_type and fails
	// hard if the extension hasn't been installed yet. CREATE EXTENSION IF
	// NOT EXISTS is idempotent and cheap on subsequent connections.
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
			return fmt.Errorf("ensure vector extension: %w", err)
		}
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
		INSERT INTO notes (id, content_enc, content_plain, summary, category, tags, is_sensitive, embedding, created_at, updated_at, summary_checked)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, TRUE)`,
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

// Get fetches a single note by full ID.
func (a *PostgresAdapter) Get(ctx context.Context, id string) (*model.Note, error) {
	row := a.pool.QueryRow(ctx, `
		SELECT id, content_enc, content_plain, summary, category, tags, is_sensitive, created_at, updated_at
		FROM notes
		WHERE id = $1`, id)
	n, err := scanPgNote(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return n, err
}

// Delete removes a note by full ID.
func (a *PostgresAdapter) Delete(ctx context.Context, id string) error {
	tag, err := a.pool.Exec(ctx, `DELETE FROM notes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MatchIDs returns up to limit IDs starting with prefix. left() compares the
// prefix literally, so % and _ in the input are not wildcards.
func (a *PostgresAdapter) MatchIDs(ctx context.Context, prefix string, limit int) ([]string, error) {
	rows, err := a.pool.Query(ctx,
		`SELECT id FROM notes WHERE left(id, $1) = $2 ORDER BY id LIMIT $3`,
		len(prefix), prefix, limit)
	if err != nil {
		return nil, fmt.Errorf("match ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
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

// UncheckedSensitive returns sensitive notes whose summary has not been
// validated against their content.
func (a *PostgresAdapter) UncheckedSensitive(ctx context.Context) ([]*model.Note, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT id, content_enc, content_plain, summary, category, tags, is_sensitive, created_at, updated_at
		FROM notes
		WHERE is_sensitive AND NOT summary_checked`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPgNotes(rows, Filters{})
}

// SetSummary replaces a note's summary and marks it validated.
func (a *PostgresAdapter) SetSummary(ctx context.Context, id, summary string) error {
	tag, err := a.pool.Exec(ctx, `UPDATE notes SET summary = $1, summary_checked = TRUE WHERE id = $2`, summary, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("set summary: note %s not found", id)
	}
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
