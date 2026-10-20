package core

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BiSemaphore/inode/internal/adapters/db"
	"github.com/BiSemaphore/inode/internal/adapters/llm"
	"github.com/BiSemaphore/inode/internal/model"
	"github.com/zalando/go-keyring"
)

func TestSummaryLeaks(t *testing.T) {
	cases := []struct {
		name     string
		summary  string
		content  string
		category string
		want     bool
	}{
		{"api key copied", "Stripe key sk_test_51HxYzAbC", "My Stripe test key is sk_test_51HxYzAbC", "credentials", true},
		{"truncated copy of long token", "GitHub PAT ghp_abcd1234ef…", "github pat ghp_abcd1234efgh5678ijkl", "credentials", true},
		{"case-insensitive", "token GHP_ABCD1234EFGH", "ghp_abcd1234efgh", "credentials", true},
		{"password with digit", "db password hunter2-xyz", "prod db password is hunter2-xyz", "credentials", true},
		{"short credential in credentials category", "wifi pass a1b2c3", "office wifi password a1b2c3", "credentials", true},
		{"long plain credential", "vault phrase correcthorsebattery", "vault unlock phrase correcthorsebattery", "credentials", true},
		{"purpose only", "Stripe test secret key", "My Stripe test key is sk_test_51HxYzAbC", "credentials", false},
		{"ordinary long words survive", "kubectl rollout restart for kubernetes deployments", "kubectl rollout restart deployment/api -n production on kubernetes", "runbooks", false},
		{"url host is not a secret", "GitHub Actions docs", "https://docs.github.com/en/actions", "references", false},
		{"url query token is a secret", "webhook token=Zx9kQ2mP7vLw", "https://hooks.example.com/in?token=Zx9kQ2mP7vLw", "references", true},
		{"empty summary", "", "sk_test_51HxYzAbC", "credentials", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := summaryLeaks(c.summary, c.content, c.category); got != c.want {
				t.Errorf("summaryLeaks(%q, %q) = %v, want %v (tokens %v)", c.summary, c.content, got, c.want, secretTokens(c.content, c.category))
			}
		})
	}
}

func TestNeutralSummary(t *testing.T) {
	if got := neutralSummary("credentials", []string{"stripe", "payment"}); got != "credentials (sensitive): stripe, payment" {
		t.Errorf("got %q", got)
	}
	if got := neutralSummary("notes", nil); got != "notes (sensitive)" {
		t.Errorf("got %q", got)
	}
	// A tag that is itself secret-looking is dropped.
	if got := neutralSummary("credentials", []string{"stripe", "sk_live_9x8y7z"}); got != "credentials (sensitive): stripe" {
		t.Errorf("got %q", got)
	}
	if got := neutralSummary("", nil); got != "item (sensitive)" {
		t.Errorf("got %q", got)
	}
}

func TestSafeSummary_NonSensitiveUnchanged(t *testing.T) {
	s := "export AWS_SECRET=abc123xyz789"
	if got := safeSummary(s, s, "commands", nil, false); got != s {
		t.Errorf("non-sensitive summary changed: %q", got)
	}
}

// summarizeLLM drives the "everything overridden" tagger path.
type summarizeLLM struct {
	stubLLM
	summary string
	err     error
}

func (s *summarizeLLM) Summarize(_ context.Context, _ string) (string, error) {
	return s.summary, s.err
}

func TestTagger_SensitiveSummarizeFailureDoesNotUseContent(t *testing.T) {
	secret := "prod db password is hunter2-xyz"
	tagger := NewTaggerService(&summarizeLLM{err: errors.New("llm down")})
	yes := true
	got, err := tagger.Classify(context.Background(), secret, ClassifyOptions{
		Category: "credentials", Tags: []string{"db"}, IsSensitive: &yes,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.Summary, "hunter2") {
		t.Fatalf("summary leaks the secret: %q", got.Summary)
	}
	if got.Summary != "credentials (sensitive): db" {
		t.Errorf("summary = %q, want neutral form", got.Summary)
	}
}

func TestTagger_NonSensitiveSummarizeFailureKeepsFirstLine(t *testing.T) {
	tagger := NewTaggerService(&summarizeLLM{err: errors.New("llm down")})
	no := false
	got, err := tagger.Classify(context.Background(), "docker system prune -a\nfrees space", ClassifyOptions{
		Category: "commands", Tags: []string{"docker"}, IsSensitive: &no,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "docker system prune -a" {
		t.Errorf("summary = %q, want first line", got.Summary)
	}
}

func TestTagger_SensitiveLeakingLLMSummaryIsReplaced(t *testing.T) {
	stub := &stubLLM{classify: llm.ClassifyResult{
		Category: "credentials", Tags: []string{"stripe"}, IsSensitive: true,
		Summary: "Stripe key sk_test_51HxYzAbC",
	}}
	got, err := NewTaggerService(stub).Classify(context.Background(), "my stripe key sk_test_51HxYzAbC", ClassifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "credentials (sensitive): stripe" {
		t.Errorf("summary = %q, want neutral form", got.Summary)
	}
}

func TestTagger_SensitiveSafeLLMSummaryIsKept(t *testing.T) {
	stub := &stubLLM{classify: llm.ClassifyResult{
		Category: "credentials", Tags: []string{"stripe"}, IsSensitive: true,
		Summary: "Stripe test secret key",
	}}
	got, err := NewTaggerService(stub).Classify(context.Background(), "my stripe key sk_test_51HxYzAbC", ClassifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "Stripe test secret key" {
		t.Errorf("summary = %q, want the LLM summary", got.Summary)
	}
}

// TestScrubSensitiveSummaries builds a database exactly as v0.4.0 left it,
// opens it (migrations add summary_checked), and runs the one-time scrub.
func TestScrubSensitiveSummaries(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	km, err := NewKeyManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	key, err := km.DeriveKey()
	if err != nil {
		t.Fatal(err)
	}
	enc := func(s string) []byte {
		b, err := Encrypt(key, []byte(s), []byte("note"))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	path := filepath.Join(dir, "notes.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		`CREATE TABLE notes (id TEXT PRIMARY KEY, content_enc BLOB, content_plain TEXT,
			summary TEXT NOT NULL DEFAULT '', category TEXT NOT NULL DEFAULT 'notes',
			tags TEXT NOT NULL DEFAULT '[]', is_sensitive INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE VIRTUAL TABLE note_embeddings USING vec0(note_id TEXT PRIMARY KEY, embedding FLOAT[4])`,
	} {
		if _, err := raw.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	insert := func(id string, contentEnc []byte, plain, summary, category string, sensitive int) {
		if _, err := raw.Exec(`INSERT INTO notes (id, content_enc, content_plain, summary, category, tags, is_sensitive, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, '["stripe"]', ?, '2026-06-01T00:00:00Z', '2026-06-01T00:00:00Z')`,
			id, contentEnc, plain, summary, category, sensitive); err != nil {
			t.Fatal(err)
		}
	}
	insert("leaky", enc("stripe live key sk_live_9x8y7zQ1"), "", "Stripe key sk_live_9x8y7zQ1", "credentials", 1)
	insert("fallback", enc("prod db password is hunter2-xyz"), "", "prod db password is hunter2-xyz", "credentials", 1)
	insert("clean", enc("stripe test key sk_test_51HxYzAbC"), "", "Stripe test secret key", "credentials", 1)
	insert("public", nil, "export TOKEN=abc123xyz789", "export TOKEN=abc123xyz789", "commands", 0)
	_ = raw.Close()

	store, err := db.NewSQLiteAdapter(path, 4)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	defer func() { _ = store.Close() }()
	svc := NewNoteService(store, nil, nil, km)

	ctx := context.Background()
	n, err := svc.ScrubSensitiveSummaries(ctx)
	if err != nil {
		t.Fatalf("scrub: %v", err)
	}
	if n != 2 {
		t.Errorf("replaced %d summaries, want 2", n)
	}

	want := map[string]string{
		"leaky":    "credentials (sensitive): stripe",
		"fallback": "credentials (sensitive): stripe",
		"clean":    "Stripe test secret key",
		"public":   "export TOKEN=abc123xyz789", // non-sensitive notes are never touched
	}
	for id, w := range want {
		got, err := store.Get(ctx, id)
		if err != nil {
			t.Fatalf("get %s: %v", id, err)
		}
		if got.Summary != w {
			t.Errorf("%s: summary = %q, want %q", id, got.Summary, w)
		}
	}

	// Second run: everything is marked checked, nothing to do.
	if n, err := svc.ScrubSensitiveSummaries(ctx); err != nil || n != 0 {
		t.Errorf("second scrub = %d, %v; want 0, nil", n, err)
	}
	left, err := store.UncheckedSensitive(ctx)
	if err != nil || len(left) != 0 {
		t.Errorf("unchecked after scrub = %d, %v", len(left), err)
	}

	// Notes saved after the upgrade are born checked.
	if _, err := store.Save(ctx, &model.Note{
		ContentEnc: enc("new secret"), Summary: "new", Category: "credentials", IsSensitive: true,
	}); err != nil {
		t.Fatal(err)
	}
	if left, _ := store.UncheckedSensitive(ctx); len(left) != 0 {
		t.Errorf("new note should not need checking, got %d unchecked", len(left))
	}
}
