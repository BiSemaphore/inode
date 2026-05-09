package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/shahid-io/inode/internal/model"
)

func init() {
	sqlite_vec.Auto() // register vec0 virtual table with the sqlite3 driver
}

// SQLiteAdapter implements DBAdapter using SQLite + sqlite-vec.
type SQLiteAdapter struct {
	db        *sql.DB
	dimension int
}

// NewSQLiteAdapter opens (or creates) the SQLite database and runs migrations.
// dimension is the embedding vector size (e.g. 768 for nomic-embed-text, 1024 for voyage-3).
func NewSQLiteAdapter(path string, dimension int) (*SQLiteAdapter, error) {
	if dimension <= 0 {
		dimension = 768
	}
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	a := &SQLiteAdapter{db: db, dimension: dimension}
	if err := a.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return a, nil
}

// migrate creates the notes table and the sqlite-vec virtual table.
func (a *SQLiteAdapter) migrate() error {
	_, err := a.db.Exec(`
		CREATE TABLE IF NOT EXISTS notes (
			id            TEXT PRIMARY KEY,
			content_enc   BLOB,
			content_plain TEXT,
			summary       TEXT    NOT NULL DEFAULT '',
			category      TEXT    NOT NULL DEFAULT 'notes',
			tags          TEXT    NOT NULL DEFAULT '[]',
			is_sensitive  INTEGER NOT NULL DEFAULT 1,
			created_at    TEXT    NOT NULL,
			updated_at    TEXT    NOT NULL
		);`)
	if err != nil {
		return err
	}

	_, err = a.db.Exec(fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS note_embeddings USING vec0(
			note_id   TEXT PRIMARY KEY,
			embedding FLOAT[%d]
		);`, a.dimension))
	return err
}

// Save persists a note and its embedding. Returns the assigned UUID.
func (a *SQLiteAdapter) Save(ctx context.Context, note *model.Note) (string, error) {
	if note.ID == "" {
		note.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	note.CreatedAt = time.Now().UTC()
	note.UpdatedAt = note.CreatedAt

	tagsJSON, err := json.Marshal(note.Tags)
	if err != nil {
		return "", err
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO notes (id, content_enc, content_plain, summary, category, tags, is_sensitive, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		note.ID,
		note.ContentEnc,
		note.ContentPlain,
		note.Summary,
		note.Category,
		string(tagsJSON),
		boolToInt(note.IsSensitive),
		now,
		now,
	)
	if err != nil {
		return "", fmt.Errorf("insert note: %w", err)
	}

	if len(note.Embedding) > 0 {
		vecBytes, err := sqlite_vec.SerializeFloat32(note.Embedding)
		if err != nil {
			return "", fmt.Errorf("serialize embedding: %w", err)
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO note_embeddings (note_id, embedding) VALUES (?, ?)`,
			note.ID, vecBytes,
		)
		if err != nil {
			return "", fmt.Errorf("insert embedding: %w", err)
		}
	}

	return note.ID, tx.Commit()
}

// Get fetches a single note by ID prefix.
func (a *SQLiteAdapter) Get(ctx context.Context, id string) (*model.Note, error) {
	row := a.db.QueryRowContext(ctx, `
		SELECT id, content_enc, content_plain, summary, category, tags, is_sensitive, created_at, updated_at
		FROM notes WHERE id LIKE ? || '%' LIMIT 1`, id)

	return scanNote(row)
}

// Delete removes a note and its embedding by ID prefix.
func (a *SQLiteAdapter) Delete(ctx context.Context, id string) error {
	// Resolve prefix to full ID first.
	var fullID string
	err := a.db.QueryRowContext(ctx, `SELECT id FROM notes WHERE id LIKE ? || '%' LIMIT 1`, id).Scan(&fullID)
	if err != nil {
		return fmt.Errorf("note not found: %w", err)
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM note_embeddings WHERE note_id = ?`, fullID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, fullID); err != nil {
		return err
	}
	return tx.Commit()
}

// SearchSimilar returns top-K notes by L2 distance to vec, in ascending order.
// Each returned note has its Distance field populated.
func (a *SQLiteAdapter) SearchSimilar(ctx context.Context, vec []float32, topK int, filters Filters) ([]*model.Note, error) {
	vecBytes, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT e.distance, n.id, n.content_enc, n.content_plain, n.summary, n.category, n.tags, n.is_sensitive, n.created_at, n.updated_at
		FROM note_embeddings e
		JOIN notes n ON n.id = e.note_id
		WHERE e.embedding MATCH ? AND k = ?
		ORDER BY e.distance`

	rows, err := a.db.QueryContext(ctx, query, vecBytes, topK)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	defer rows.Close()

	return scanScoredNotes(rows, filters)
}

// List returns notes matching filters with pagination.
func (a *SQLiteAdapter) List(ctx context.Context, filters Filters, limit, offset int) ([]*model.Note, error) {
	query := `
		SELECT id, content_enc, content_plain, summary, category, tags, is_sensitive, created_at, updated_at
		FROM notes
		WHERE 1=1`
	args := []any{}

	if filters.Category != "" {
		query += ` AND category = ?`
		args = append(args, filters.Category)
	}
	if filters.IsSensitive != nil {
		query += ` AND is_sensitive = ?`
		args = append(args, boolToInt(*filters.IsSensitive))
	}

	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanNotes(rows, filters)
}

// Close releases the database connection.
func (a *SQLiteAdapter) Close() error {
	return a.db.Close()
}

// scanNote scans a single row into a Note.
func scanNote(row *sql.Row) (*model.Note, error) {
	var n model.Note
	var tagsJSON string
	var createdAt, updatedAt string
	var isSensitive int

	err := row.Scan(
		&n.ID, &n.ContentEnc, &n.ContentPlain,
		&n.Summary, &n.Category, &tagsJSON,
		&isSensitive, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	n.IsSensitive = isSensitive == 1
	if err := json.Unmarshal([]byte(tagsJSON), &n.Tags); err != nil {
		n.Tags = []string{}
	}
	n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	n.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &n, nil
}

// scanNotes scans multiple rows into Notes, applying tag filter in memory.
func scanNotes(rows *sql.Rows, filters Filters) ([]*model.Note, error) {
	var notes []*model.Note
	for rows.Next() {
		var n model.Note
		var tagsJSON string
		var createdAt, updatedAt string
		var isSensitive int

		err := rows.Scan(
			&n.ID, &n.ContentEnc, &n.ContentPlain,
			&n.Summary, &n.Category, &tagsJSON,
			&isSensitive, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		n.IsSensitive = isSensitive == 1
		if err := json.Unmarshal([]byte(tagsJSON), &n.Tags); err != nil {
			n.Tags = []string{}
		}
		n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		n.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		if matchesTagFilter(&n, filters.Tags) {
			notes = append(notes, &n)
		}
	}
	return notes, rows.Err()
}

// scanScoredNotes scans rows from a similarity search where the first column is the distance.
func scanScoredNotes(rows *sql.Rows, filters Filters) ([]*model.Note, error) {
	var notes []*model.Note
	for rows.Next() {
		var n model.Note
		var distance float32
		var tagsJSON string
		var createdAt, updatedAt string
		var isSensitive int

		err := rows.Scan(
			&distance,
			&n.ID, &n.ContentEnc, &n.ContentPlain,
			&n.Summary, &n.Category, &tagsJSON,
			&isSensitive, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		n.Distance = distance
		n.IsSensitive = isSensitive == 1
		if err := json.Unmarshal([]byte(tagsJSON), &n.Tags); err != nil {
			n.Tags = []string{}
		}
		n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		n.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		if matchesTagFilter(&n, filters.Tags) {
			notes = append(notes, &n)
		}
	}
	return notes, rows.Err()
}

func matchesTagFilter(note *model.Note, filterTags []string) bool {
	if len(filterTags) == 0 {
		return true
	}
	tagSet := make(map[string]bool, len(note.Tags))
	for _, t := range note.Tags {
		tagSet[t] = true
	}
	for _, ft := range filterTags {
		if tagSet[ft] {
			return true
		}
	}
	return false
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
