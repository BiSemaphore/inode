# inode — Technical Architecture

---

## 1. System Layers

```
┌──────────────────────────────────────────────────────────────────┐
│                          CLI Layer                                │
│   inode add / inode ask / inode list / inode note / inode config  │
│   Go + Cobra + Lipgloss                                           │
└──────────────────────────┬───────────────────────────────────────┘
                           │ direct calls (Phase 1)
                           │ HTTP/REST (Phase 2+)
                           ▼
┌──────────────────────────────────────────────────────────────────┐
│                       Core Services Layer                         │
│                                                                  │
│  ┌──────────────┐   ┌───────────────┐   ┌─────────────────────┐ │
│  │  NoteService │   │ SearchService │   │    TaggerService    │ │
│  │  add/get/del │   │ RAG pipeline  │   │  auto-tag via LLM   │ │
│  └──────┬───────┘   └───────┬───────┘   └──────────┬──────────┘ │
│         │                   │                       │            │
│  ┌──────▼───────┐   ┌───────▼───────┐   ┌──────────▼──────────┐ │
│  │  Encryption  │   │   Embedding   │   │    LLM Adapter      │ │
│  │  AES-256-GCM │   │   Adapter     │   │  Claude / Ollama    │ │
│  └──────────────┘   └───────────────┘   └─────────────────────┘ │
└──────────────────────────┬───────────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────────┐
│                        Storage Layer                              │
│                                                                  │
│  Phase 1: SQLite + sqlite-vec     ~/.inode/notes.db              │
│  Phase 2: PostgreSQL + pgvector   (same DBAdapter interface)     │
└──────────────────────────────────────────────────────────────────┘
```

---

## 2. Folder Structure

Follows standard Go project layout: `cmd/` for CLI entry points,
`internal/` for all private packages (not importable by external code).

```
inode/
├── main.go                        # entry point — calls cmd.Execute()
├── go.mod
├── go.sum
│
├── cmd/                           # Cobra commands (CLI layer)
│   ├── root.go                    # root command, global flags, config init
│   ├── add.go                     # inode add
│   ├── ask.go                     # inode ask
│   ├── list.go                    # inode list
│   ├── note.go                    # inode note get/edit/delete
│   └── config.go                  # inode config set/show
│
├── internal/
│   ├── core/
│   │   ├── notes.go               # NoteService — orchestrates add/get/delete
│   │   ├── search.go              # SearchService — RAG pipeline
│   │   ├── tagger.go              # TaggerService — LLM classification
│   │   └── encryption.go          # AES-256-GCM + Argon2id key derivation
│   │
│   ├── adapters/
│   │   ├── llm/
│   │   │   ├── adapter.go         # LLMAdapter interface
│   │   │   ├── claude.go          # Claude API implementation
│   │   │   └── ollama.go          # Ollama implementation (Phase 3)
│   │   ├── embedding/
│   │   │   ├── adapter.go         # EmbeddingAdapter interface
│   │   │   └── voyage.go          # Voyage AI HTTP client
│   │   └── db/
│   │       ├── adapter.go         # DBAdapter interface
│   │       ├── sqlite.go          # Phase 1: SQLite + sqlite-vec
│   │       └── postgres.go        # Phase 2: PostgreSQL + pgvector
│   │
│   ├── config/
│   │   └── settings.go            # Read/write ~/.inode/config.toml
│   │
│   └── model/
│       └── note.go                # Note struct, shared types
│
└── server/                        # Phase 2 only — Go + chi HTTP server
    ├── main.go
    ├── routes/
    │   ├── notes.go
    │   ├── search.go
    │   └── auth.go
    └── middleware/
        └── auth.go                # JWT verification
```

---

## 3. Adapter Interfaces

The three abstraction boundaries that keep inode's backend swappable.
All interfaces live in `internal/adapters/*/adapter.go`.

### LLM Adapter

```go
// internal/adapters/llm/adapter.go

type ClassifyResult struct {
    Category    string
    Tags        []string
    IsSensitive bool
    Summary     string
}

type LLMAdapter interface {
    // Classify returns category, tags, sensitivity, and a one-line summary.
    // Called when a note is added without explicit metadata.
    Classify(ctx context.Context, content string, categories []Category, tags []string) (ClassifyResult, error)

    // Answer performs RAG generation: query + retrieved notes → natural language answer.
    Answer(ctx context.Context, query string, notes []model.Note) (string, error)

    // Summarize returns a one-line description of the note content.
    Summarize(ctx context.Context, content string) (string, error)
}
```

### Embedding Adapter

```go
// internal/adapters/embedding/adapter.go

type EmbeddingAdapter interface {
    // Embed converts text to a float32 vector.
    // Voyage AI (voyage-3) returns 1024 dimensions.
    // All embeddings in the DB must use the same model.
    Embed(ctx context.Context, text string) ([]float32, error)
}
```

### DB Adapter

```go
// internal/adapters/db/adapter.go

type Filters struct {
    Category    string
    Tags        []string
    IsSensitive *bool
}

type DBAdapter interface {
    Save(ctx context.Context, note *model.Note) (string, error)
    Get(ctx context.Context, id string) (*model.Note, error)
    Delete(ctx context.Context, id string) error

    // SearchSimilar returns top-K notes by cosine similarity to vec,
    // optionally filtered by Filters.
    SearchSimilar(ctx context.Context, vec []float32, topK int, filters Filters) ([]*model.Note, error)

    List(ctx context.Context, filters Filters, limit, offset int) ([]*model.Note, error)
}
```

---

## 4. Data Flow: `inode add`

```
User: inode add "My Stripe test key is sk_test_xxxxx"

Step 1 — cmd/add.go
  Parse arguments. No category/tags provided → auto-detect mode.

Step 2 — TaggerService.Classify(content)
  LLMAdapter.Classify(content, predefined_categories, predefined_tags)
  → Claude API call:
      "Classify this note given these categories: [...]
       Return JSON: { category, tags, is_sensitive, summary }"
  ← { category: "credentials", tags: ["stripe", "payment"],
      is_sensitive: true, summary: "Stripe test secret key" }

Step 3 — EmbeddingAdapter.Embed(content)
  → Voyage AI HTTP call (POST /embeddings)
  ← []float32{0.124, 0.876, ..., 0.341}   (1024 values)

Step 4 — Encryption (because is_sensitive=true)
  key = Argon2id(OS_keychain_secret, config_salt)
  iv  = 12 random bytes (crypto/rand)
  ciphertext, auth_tag = AES_256_GCM_Seal(key, iv, []byte(content), noteID)
  note.ContentEnc  = iv + auth_tag + ciphertext  (single []byte)
  note.ContentPlain = ""

Step 5 — DBAdapter.Save(note)
  INSERT INTO notes (id, content_enc, summary, category, tags,
                     is_sensitive, created_at, updated_at)
  INSERT INTO note_embeddings (note_id, embedding)

Step 6 — CLI output (Lipgloss formatted)
  ✓ Auto-detected: category=credentials, tags=[stripe, payment]
  ✓ Flagged as: sensitive=true
  ✓ Saved as note abc123
```

---

## 5. Data Flow: `inode ask`

```
User: inode ask "stripe test key"

Step 1 — cmd/ask.go
  Parse query string. Check for --reveal flag.

Step 2 — EmbeddingAdapter.Embed(query)
  → Voyage AI HTTP call
  ← queryVec []float32 (1024 values)

Step 3 — DBAdapter.SearchSimilar(queryVec, topK=5, filters={})
  SQL (sqlite-vec):
    SELECT n.*, vec_distance_cosine(e.embedding, ?) AS score
    FROM notes n
    JOIN note_embeddings e ON e.note_id = n.id
    ORDER BY score ASC
    LIMIT 5
  ← []*model.Note — top 5 matches

Step 4 — Decrypt sensitive notes in memory
  For each note where IsSensitive=true:
    plaintext = AES_256_GCM_Open(key, note.ContentEnc, noteID)
  Plaintext never written to disk.

Step 5 — SearchService builds context string
  Join decrypted note content for top-K notes.

Step 6 — LLMAdapter.Answer(query, notes)
  → Claude API call:
      "Answer using only these notes as context: [...]
       User query: stripe test key"
  ← "Your Stripe test secret key is in note abc123."

Step 7 — cmd/ask.go output
  --reveal not set → mask sensitive values (sk_test_******)
  --reveal set     → print "Reveal sensitive value? [y/N]"
                     on 'y': print full plaintext
```

---

## 6. Encryption Design

```
First run (inode config init):
  secret ← crypto/rand, 32 bytes → stored in OS keychain (go-keyring)
  salt   ← crypto/rand, 16 bytes → stored in ~/.inode/config.toml (base64)

Key derivation (per process, result never persisted):
  key = argon2.IDKey(
    password = secret,         // from OS keychain
    salt     = salt,           // from config
    time     = 2,
    memory   = 64 * 1024,      // 64MB
    threads  = 1,
    keyLen   = 32,
  )

Encrypting a note:
  iv        = 12 random bytes (crypto/rand, new per note)
  aead, _  := aes.NewCipher(key) → cipher.NewGCM(aead)
  sealed   := gcm.Seal(nil, iv, []byte(plaintext), []byte(noteID))
  // sealed = ciphertext + 16-byte auth tag (appended by GCM)

Stored in DB as single BLOB:
  []byte{ iv[0:12] | sealed... }   (iv + ciphertext + auth_tag)

Decrypting:
  iv       = blob[0:12]
  sealed   = blob[12:]
  plain, _ = gcm.Open(nil, iv, sealed, []byte(noteID))
  // GCM Open verifies auth tag — returns error if tampered
```

**Linux headless fallback:** When `go-keyring` fails (no desktop session / WSL /
headless SSH), inode falls back to a key file at `~/.inode/.key` with `chmod 600`.
This is warned about on first run.

---

## 7. RAG Pipeline

inode is a RAG (Retrieval-Augmented Generation) system. Three stages:

**Indexing** — at `inode add`:
```
note content → Embed() → float32 vector stored in DB alongside note
```

**Retrieval** — at `inode ask`:
```
user query → Embed() → cosine similarity search → top-K notes returned
```

**Generation** — at `inode ask`:
```
top-K notes injected as context → Claude answers using YOUR data only
```

Claude never sees the full notes database — only the top-K most relevant notes.
This keeps latency low, cost minimal, and context tightly focused.

**Why cosine similarity?** It measures the angle between vectors, not magnitude.
Notes of different lengths expressing the same meaning will score close together.
Euclidean distance penalises length differences and performs worse for text.

---

## 8. Auto-Tagging Flow

```
                       User adds note
                             │
              ┌──────────────▼──────────────┐
              │  Did user specify category   │
              │       AND tags?              │
              └──────┬───────────────┬───────┘
                   YES               NO
                     │               │
                     ▼               ▼
             Validate against   Send to Claude:
             predefined list    "Classify this note given
             (fuzzy match via   categories: [...] tags: [...]"
              Claude if needed)
                     │               │
                     └───────┬───────┘
                             ▼
                  { category, tags, is_sensitive, summary }
                             │
                             ▼
                   Store note with metadata + embedding
```

---

## 9. Phase 2: CLI → Server Transition

In Phase 2, the CLI stops calling `internal/core` directly and issues HTTP requests
to a Go + chi server. The core service layer and all adapters are unchanged —
they move into the server process. Same language, same packages, no rewrite.

```
Phase 1:                              Phase 2:
┌─────────┐                           ┌─────────┐   HTTPS   ┌──────────────┐
│   CLI   │──▶ internal/core ──▶ DB   │   CLI   │──────────▶│  chi Server  │
└─────────┘                           └─────────┘           │              │
                                                            │ internal/core│
                                                            │ adapters/    │
                                                            │ PostgreSQL   │
                                                            └──────────────┘
```

The adapter interfaces are the only seam. Swapping `sqlite.go` for `postgres.go`
is the entire DB migration. NoteService, SearchService, TaggerService — untouched.

---

## 10. Phase 1 SQLite Schema

```sql
CREATE TABLE notes (
    id            TEXT PRIMARY KEY,    -- UUID v4
    content_enc   BLOB,                -- iv + ciphertext + auth_tag (null if not sensitive)
    content_plain TEXT,                -- plaintext (null if sensitive)
    summary       TEXT    NOT NULL,    -- LLM-generated one-liner
    category      TEXT    NOT NULL,
    tags          TEXT    NOT NULL,    -- JSON array e.g. '["stripe","payment"]'
    is_sensitive  INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT    NOT NULL,    -- RFC3339
    updated_at    TEXT    NOT NULL
);

-- sqlite-vec virtual table for ANN similarity search
CREATE VIRTUAL TABLE note_embeddings USING vec0(
    note_id   TEXT PRIMARY KEY,
    embedding FLOAT[1024]              -- voyage-3 produces 1024-dim vectors
);
```

---

## 11. Config Resolution Order

```
1. CLI flags          --sensitive, --category, --tags, --reveal
2. Environment vars   INODE_LLM_BACKEND, INODE_LLM_MODEL, INODE_API_KEY
3. ~/.inode/config.toml
4. Hardcoded defaults
```

---

## 12. Go Module Dependencies

| Module | Purpose | Phase |
|---|---|---|
| `github.com/anthropics/anthropic-sdk-go` | Claude API | 1 |
| `github.com/spf13/cobra` | CLI framework | 1 |
| `github.com/charmbracelet/lipgloss` | Terminal styling | 1 |
| `github.com/charmbracelet/bubbles` | Spinners, tables, prompts | 1 |
| `github.com/asg017/sqlite-vec-go-bindings` | sqlite-vec vector search | 1 |
| `github.com/mattn/go-sqlite3` | SQLite driver (CGO) | 1 |
| `github.com/zalando/go-keyring` | OS keychain (Mac/Linux/Windows) | 1 |
| `github.com/BurntSushi/toml` | TOML config parsing | 1 |
| `github.com/google/uuid` | UUID generation | 1 |
| `golang.org/x/crypto/argon2` | Argon2id key derivation | 1 |
| `github.com/go-chi/chi/v5` | HTTP router | 2 |
| `github.com/jackc/pgx/v5` | PostgreSQL driver | 2 |
| `github.com/pgvector/pgvector-go` | pgvector client | 2 |
| `github.com/golang-jwt/jwt/v5` | JWT auth | 2 |
| `github.com/pressly/goose/v3` | DB migrations | 2 |
