package core

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BiSemaphore/inode/internal/adapters/db"
	"github.com/BiSemaphore/inode/internal/model"
)

// newResolveService returns a NoteService over a real SQLite database holding
// notes with the given IDs. Resolve and non-sensitive Get need no key manager.
func newResolveService(t *testing.T, notes ...*model.Note) *NoteService {
	t.Helper()
	store, err := db.NewSQLiteAdapter(filepath.Join(t.TempDir(), "notes.db"), 4)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for _, n := range notes {
		n.Embedding = []float32{1, 0, 0, 0}
		if _, err := store.Save(context.Background(), n); err != nil {
			t.Fatalf("save %s: %v", n.ID, err)
		}
	}
	return NewNoteService(store, nil, nil, nil)
}

func TestResolve(t *testing.T) {
	svc := newResolveService(t,
		&model.Note{ID: "3f1a9c02-1111", Summary: "docker prune", Category: "commands"},
		&model.Note{ID: "3b77e410-2222", Summary: "Stripe test secret key", Category: "credentials", IsSensitive: true},
		&model.Note{ID: "c0de0000-3333", Summary: "go test flags", Category: "commands"},
	)
	ctx := context.Background()

	for _, ref := range []string{"c0de", "C0DE", " c0de0000-3333 ", "c0de0000-3333"} {
		n, err := svc.Resolve(ctx, ref)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", ref, err)
		}
		if n.ID != "c0de0000-3333" {
			t.Errorf("Resolve(%q) = %s, want c0de0000-3333", ref, n.ID)
		}
	}

	for _, ref := range []string{"", "  ", "%", "3%", "c0de_", "zz"} {
		if _, err := svc.Resolve(ctx, ref); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Resolve(%q) error = %v, want ErrInvalidID", ref, err)
		}
	}

	if _, err := svc.Resolve(ctx, "ffff"); !errors.Is(err, db.ErrNotFound) {
		t.Errorf("Resolve(ffff) error = %v, want db.ErrNotFound", err)
	}

	_, err := svc.Resolve(ctx, "3")
	var amb *AmbiguousIDError
	if !errors.As(err, &amb) {
		t.Fatalf("Resolve(3) error = %v, want *AmbiguousIDError", err)
	}
	if len(amb.Matches) != 2 || amb.More {
		t.Errorf("matches = %d, more = %v; want 2, false", len(amb.Matches), amb.More)
	}
	msg := err.Error()
	for _, want := range []string{"#3f1a9c02  docker prune [commands]", "#3b77e410  (sensitive) [credentials]"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not contain %q", msg, want)
		}
	}
	if strings.Contains(msg, "Stripe") {
		t.Errorf("error leaks a sensitive note's summary: %q", msg)
	}
}

func TestResolve_ManyMatchesAreCapped(t *testing.T) {
	var notes []*model.Note
	for _, id := range []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7"} {
		notes = append(notes, &model.Note{ID: id + "000000-0000", Summary: id, Category: "notes"})
	}
	svc := newResolveService(t, notes...)

	_, err := svc.Resolve(context.Background(), "a")
	var amb *AmbiguousIDError
	if !errors.As(err, &amb) {
		t.Fatalf("error = %v, want *AmbiguousIDError", err)
	}
	if len(amb.Matches) != maxMatches || !amb.More {
		t.Errorf("matches = %d, more = %v; want %d, true", len(amb.Matches), amb.More, maxMatches)
	}
}

func TestGet_AmbiguousPrefixReturnsNoNote(t *testing.T) {
	svc := newResolveService(t,
		&model.Note{ID: "3f1a9c02-1111", Summary: "a", Category: "notes"},
		&model.Note{ID: "3b77e410-2222", Summary: "b", Category: "notes"},
	)
	n, err := svc.Get(context.Background(), "3")
	var amb *AmbiguousIDError
	if n != nil || !errors.As(err, &amb) {
		t.Errorf("Get(3) = %v, %v; want nil, *AmbiguousIDError", n, err)
	}
}
