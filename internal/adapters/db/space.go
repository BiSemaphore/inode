package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/jackc/pgx/v5"
)

// Embedding space bookkeeping (#130): which model made the stored vectors,
// and the vector size the schema was created with.

func (a *SQLiteAdapter) EmbeddingSpace(ctx context.Context) (EmbeddingSpace, bool, error) {
	var s EmbeddingSpace
	err := a.db.QueryRowContext(ctx, `SELECT backend, model, dimension FROM embedding_meta WHERE id = 1`).
		Scan(&s.Backend, &s.Model, &s.Dimension)
	if errors.Is(err, sql.ErrNoRows) {
		return EmbeddingSpace{}, false, nil
	}
	if err != nil {
		return EmbeddingSpace{}, false, fmt.Errorf("read embedding_meta: %w", err)
	}
	return s, true, nil
}

func (a *SQLiteAdapter) SetEmbeddingSpace(ctx context.Context, s EmbeddingSpace) error {
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO embedding_meta (id, backend, model, dimension) VALUES (1, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET backend = excluded.backend, model = excluded.model, dimension = excluded.dimension`,
		s.Backend, s.Model, s.Dimension)
	if err != nil {
		return fmt.Errorf("write embedding_meta: %w", err)
	}
	return nil
}

var vec0Dimension = regexp.MustCompile(`(?i)embedding\s+FLOAT\[(\d+)\]`)

// VectorDimension reads the size from the vec0 table definition, since
// sqlite-vec exposes no other way to ask for it.
func (a *SQLiteAdapter) VectorDimension(ctx context.Context) (int, error) {
	var ddl string
	err := a.db.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE name = 'note_embeddings'`).Scan(&ddl)
	if err != nil {
		return 0, fmt.Errorf("read note_embeddings definition: %w", err)
	}
	m := vec0Dimension.FindStringSubmatch(ddl)
	if m == nil {
		return 0, fmt.Errorf("no vector size in note_embeddings definition %q", ddl)
	}
	return strconv.Atoi(m[1])
}

// ResizeVectors recreates the vec0 table, which cannot be altered in place.
func (a *SQLiteAdapter) ResizeVectors(ctx context.Context, dimension int) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM note_embeddings`).Scan(&n); err != nil {
		return fmt.Errorf("count embeddings: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("cannot change the vector size: %d notes have embeddings", n)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE note_embeddings`); err != nil {
		return fmt.Errorf("drop note_embeddings: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`CREATE VIRTUAL TABLE note_embeddings USING vec0(
			note_id   TEXT PRIMARY KEY,
			embedding FLOAT[%d]
		)`, dimension)); err != nil {
		return fmt.Errorf("create note_embeddings: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	a.dimension = dimension
	return nil
}

func (a *SQLiteAdapter) CountNotes(ctx context.Context) (int, error) {
	var n int
	if err := a.db.QueryRowContext(ctx, `SELECT count(*) FROM notes`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count notes: %w", err)
	}
	return n, nil
}

func (a *PostgresAdapter) EmbeddingSpace(ctx context.Context) (EmbeddingSpace, bool, error) {
	var s EmbeddingSpace
	err := a.pool.QueryRow(ctx, `SELECT backend, model, dimension FROM embedding_meta WHERE id = 1`).
		Scan(&s.Backend, &s.Model, &s.Dimension)
	if errors.Is(err, pgx.ErrNoRows) {
		return EmbeddingSpace{}, false, nil
	}
	if err != nil {
		return EmbeddingSpace{}, false, fmt.Errorf("read embedding_meta: %w", err)
	}
	return s, true, nil
}

func (a *PostgresAdapter) SetEmbeddingSpace(ctx context.Context, s EmbeddingSpace) error {
	_, err := a.pool.Exec(ctx, `
		INSERT INTO embedding_meta (id, backend, model, dimension) VALUES (1, $1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET backend = excluded.backend, model = excluded.model, dimension = excluded.dimension`,
		s.Backend, s.Model, s.Dimension)
	if err != nil {
		return fmt.Errorf("write embedding_meta: %w", err)
	}
	return nil
}

// VectorDimension reads the type modifier of notes.embedding, which for
// pgvector's vector(n) is n itself.
func (a *PostgresAdapter) VectorDimension(ctx context.Context) (int, error) {
	var n int
	err := a.pool.QueryRow(ctx, `
		SELECT atttypmod FROM pg_attribute
		WHERE attrelid = 'notes'::regclass AND attname = 'embedding'`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("read notes.embedding type: %w", err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("notes.embedding has no vector size (typmod %d)", n)
	}
	return n, nil
}

func (a *PostgresAdapter) ResizeVectors(ctx context.Context, dimension int) error {
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM notes WHERE embedding IS NOT NULL`).Scan(&n); err != nil {
		return fmt.Errorf("count embeddings: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("cannot change the vector size: %d notes have embeddings", n)
	}
	// dimension is an int, so formatting it into DDL is safe.
	if _, err := tx.Exec(ctx, fmt.Sprintf(`ALTER TABLE notes ALTER COLUMN embedding TYPE vector(%d)`, dimension)); err != nil {
		return fmt.Errorf("alter notes.embedding: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	a.dimension = dimension
	return nil
}

func (a *PostgresAdapter) CountNotes(ctx context.Context) (int, error) {
	var n int
	if err := a.pool.QueryRow(ctx, `SELECT count(*) FROM notes`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count notes: %w", err)
	}
	return n, nil
}
