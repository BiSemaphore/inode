# inode architecture

How inode is built, as of v0.5.1. Every section names the files it describes;
if the code and this document disagree, the code wins and this document has a
bug. Plans for teams and organizations are not here; see [What's next](#whats-next).

## 1. Layers

```
  cmd/                 Cobra commands: parse flags, print results, ask for confirmation
   |
   v
  internal/core/       Container (wiring), NoteService, SearchService, TaggerService,
   |                   KeyManager (encryption), summary checks
   v
  internal/adapters/   db         sqlite.go (sqlite-vec), postgres.go (pgvector)
                       llm        ollama.go, claude.go, openai.go
                       embedding  ollama.go, voyage.go

  internal/mcp/        MCP server over stdio; uses core, read-only
  internal/config/     Config loading (viper): file, environment, defaults
  internal/model/      Note, Category
  internal/cli/        Terminal spinner
  internal/version/    Version string (set by ldflags at release)
```

`cmd` and `internal/mcp` are the two front ends. Both reach storage and the
models only through the services on `core.Container` (`Notes`, `Search`);
neither calls an adapter directly.

## 2. Packages

| Package | Files | Responsibility |
|---|---|---|
| `cmd` | `root.go`, `add.go`, `get.go`, `list.go`, `note.go`, `config.go`, `mcp.go` | Commands, flags, output formatting, confirmation prompts |
| `internal/core` | `container.go` | Builds every adapter and service from the config; runs the one-time summary check |
| | `notes.go` | `NoteService`: add (classify, embed, encrypt, save), get (decrypt), list, delete |
| | `search.go` | `SearchService`: embed the query, vector search, distance filter, decrypt, LLM answer; `MaskSensitive` |
| | `tagger.go` | `TaggerService`: category, tags, sensitivity and summary, explicit flags first |
| | `summary.go` | Keeps secrets out of summaries of sensitive notes (#53) |
| | `encryption.go` | `KeyManager` (master secret, salt, Argon2id), `Encrypt` / `Decrypt` (AES-256-GCM) |
| `internal/adapters/db` | `adapter.go`, `sqlite.go`, `postgres.go`, `migrate.go` | Storage and vector search; schema migrations |
| `internal/adapters/llm` | `adapter.go`, `ollama.go`, `claude.go`, `openai.go` | Classify, answer, summarize |
| `internal/adapters/embedding` | `adapter.go`, `ollama.go`, `voyage.go` | Text to vector |
| `internal/mcp` | `server.go`, `tools.go` | `search_notes`, `list_notes`, `get_note` |
| `internal/config` | `settings.go`, `write.go` | `Config` struct, defaults, environment variables; writing `config.toml` at mode 0600 |
| `internal/model` | `note.go` | `Note`, the fixed category list |

## 3. Adapters

Each backend is an implementation of one interface. `core.NewContainer`
(`internal/core/container.go`) picks the implementation from the config and
fails with the list of supported values for anything else.

| Interface | Methods | Implementations | Config key |
|---|---|---|---|
| `db.Adapter` | `Save`, `Get`, `Delete`, `SearchSimilar`, `List`, `UncheckedSensitive`, `SetSummary`, `Close` | `sqlite` (default), `postgres` | `db.backend` |
| `llm.Adapter` | `Classify`, `Answer`, `Summarize` | `ollama` (default), `claude-api`, `openai` | `llm.backend` |
| `embedding.Adapter` | `Embed` | `ollama` (default), `voyage` | `embedding.backend` |

Backend details:

| Backend | Talks to | Default model | Notes |
|---|---|---|---|
| LLM `ollama` | `<llm.base_url>/api/chat` | `llama3.2` | JSON output via `"format":"json"` |
| LLM `claude-api` | Anthropic API (official Go SDK) | `claude-sonnet-5-5` when `llm.model` is empty | needs `llm.api_key` |
| LLM `openai` | `<base_url>/chat/completions`, plain `net/http` | `gpt-4o-mini` when `llm.model` is empty | JSON mode; base URL defaults to `https://api.openai.com/v1` (section 11) |
| Embedding `ollama` | `<embedding.base_url>/api/embed` | `nomic-embed-text`, 768 dimensions | |
| Embedding `voyage` | `https://api.voyageai.com/v1/embeddings` | `voyage-3`, 1024 dimensions | needs `embedding.api_key` |

Embedding adapters must return L2-normalised vectors (`embedding/adapter.go`):
the distance threshold in search assumes unit vectors. All notes in one
database must use the same embedding model and dimension; the dimension is
fixed into the schema when the database is created.

`llm.Answer` returns the answer, whether the notes contained it, and the
short IDs of the notes it used. Search shows only those notes as sources.

## 4. `inode add`

`cmd/add.go` then `NoteService.Add` (`internal/core/notes.go`):

1. **Sensitivity default.** Without `--sensitive` or `--no-sensitive`, the
   note uses `defaults.sensitive` (true by default).
2. **Classify** (`TaggerService.Classify`, `tagger.go`). If `--category`,
   `--tags` and a sensitivity flag are all given, the LLM is asked only for a
   summary. Otherwise the LLM classifies the note against the fixed category
   list (`model.Categories`) and a suggested tag list; an unknown category
   becomes `notes`. Explicit flags override the LLM's choices. Categories and
   tags are lowercased.
3. **Summary check.** For a sensitive note, a summary that is empty or copies
   a secret-looking token from the content is replaced with
   `<category> (sensitive): <tags>` (`summary.go`, #53).
4. **Embed** the full content (`embedding.Adapter.Embed`).
5. **Encrypt** if sensitive: AES-256-GCM with a key derived from the master
   secret (section 6). The plaintext is not stored.
6. **Save.** The adapter assigns a UUID; the CLI prints the first 8 characters.

Output (`cmd/add.go`):

```
  category=runbooks  tags=[vault, postgres, staging]  sensitive=true
  saved  id=a1b2c3d4
```

## 5. `inode get`

`inode get <query>` (aliases `ask`, `find`, `search`), `cmd/get.go` then
`SearchService.Search` (`internal/core/search.go`). The spinner
(`internal/cli/spinner.go`) shows each step on stderr and clears before output.

1. **embedding query**: embed the query with the same embedding backend.
2. **searching**: `db.Adapter.SearchSimilar` returns the `search.top_k`
   nearest notes (default 5) by L2 distance, then notes farther than
   `search.max_distance` (default 1.0, about cosine similarity 0.5 for unit
   vectors) are dropped. If nothing is left, the answer is
   `No relevant notes found.` and the LLM is not called.
3. Sensitive candidates are decrypted in memory for the LLM.
4. **thinking**: `llm.Answer` gets the query and the candidates and returns
   an answer plus the IDs of the notes it used. Only those are shown as sources.
5. Sources are masked unless `--reveal` is passed and confirmed.

Output:

```
Run vault write database/rotate-root/staging, then restart the api.

── Sources (1 note(s)) ──
  #a1b2c3d4  Rotate staging DB creds  [runbooks] [vault, postgres, staging]  [sensitive]
```

Other commands: `inode list` (filters `--category`, `--tag`, `--limit`;
sensitive notes only with `--sensitive` and a confirmation), `inode note get <id>`
(asks before revealing sensitive content), `inode note delete <id>`,
`inode config set|show`, `inode mcp`, `inode version`.

Note IDs can be given as a prefix. `NoteService.Resolve`
(`internal/core/notes.go`) accepts only hex digits and `-`, looks up matches
with the adapter's `MatchIDs` (a literal prefix comparison, so `%` and `_` are
not wildcards) and returns exactly one note: no match is "not found", more than
one is an error listing the candidates (sensitive ones without their summary).
Adapter `Get` and `Delete` take full IDs only. `inode note delete` needs at
least 4 characters and its prompt names the resolved note (#90).

## 6. Encryption and keys

`internal/core/encryption.go`.

| Item | Where | Details |
|---|---|---|
| Master secret | OS keychain, service `inode`, account `master-secret` (macOS Keychain, Linux Secret Service, Windows Credential Manager via `go-keyring`) | 32 random bytes, base64 |
| Fallback secret file | `~/.inode/.key`, mode 0600 | Used only when no keychain is reachable (headless Linux); a warning is printed when it is written |
| Salt | `~/.inode/.salt`, mode 0600 | Random, created on first run |
| Data key | derived per use | Argon2id(secret, salt), time 2, memory 64 MB, 1 thread, 32-byte key |
| Ciphertext | `notes.content_enc` | 12-byte random nonce followed by the AES-256-GCM sealed content |

**First-run rule (#104).** A new master secret is created only when the
keychain has no item *and* neither `.salt` nor `.key` exists. Any other
keychain error (locked keychain, denied prompt, unreachable service) stops
inode with an explanation and writes nothing, so an existing secret is never
replaced.

**Known gap (#100).** The GCM additional data is the constant `"note"`, not
the note ID, so ciphertexts are not bound to their row.

## 7. What is and is not protected for sensitive notes

| Data | Stored as | Shown |
|---|---|---|
| Content | Encrypted (section 6) | Masked as `••••••` unless revealed with confirmation (`get --reveal`, `note get`, `list --sensitive`) |
| Summary | Plaintext, checked for copied secrets (#53) | In `list`, `get` sources, `note get` |
| Category, tags | Plaintext | Everywhere |
| Embedding | Plaintext vector | Never shown; not reversible to text, but encodes meaning |

With a cloud backend (`claude-api`, `openai`, `voyage`), note content, including
sensitive content, is sent to that provider on `add` (classify, embed) and on
`get` (answer). `SECURITY.md` lists all known limitations.

## 8. Storage

### SQLite (default)

`internal/adapters/db/sqlite.go`. File `~/.inode/notes.db` (or `db.path`),
opened with WAL journaling. Vectors live in a `vec0` virtual table from
[sqlite-vec](https://github.com/asg017/sqlite-vec) (CGO). Similarity search is
a KNN query (`embedding MATCH ? AND k = ?`) ordered by L2 distance; category,
tag and sensitivity filters are applied to the K results in Go, so a filtered
search can return fewer than K notes.

### Postgres

`internal/adapters/db/postgres.go`, selected with `db.backend = postgres` and
`db.dsn`. Uses `pgx` and [pgvector](https://github.com/pgvector/pgvector);
`CREATE EXTENSION IF NOT EXISTS vector` runs on connect. The embedding is a
`vector(<dimension>)` column on `notes`; search orders by the `<->` (L2)
operator with filters in the SQL `WHERE` clause.

## 9. Schema and migrations

`internal/adapters/db/migrate.go` (#85). Migrations are numbered from 1 with
no gaps and run in order when the adapter opens the database.

| Version | SQLite | Postgres |
|---|---|---|
| 1 | `notes` (id, content_enc, content_plain, summary, category, tags JSON text, is_sensitive, created_at, updated_at); `note_embeddings` vec0 table (note_id, embedding FLOAT[dim]) | `notes` with the same columns, `tags JSONB`, `embedding vector(dim)` |
| 2 | `summary_checked` column; non-sensitive rows marked checked | same |

| | SQLite | Postgres |
|---|---|---|
| Applied version | `PRAGMA user_version` | `schema_migrations (version, name, applied_at)` |
| Atomicity | One transaction per migration | All pending migrations in one transaction |
| Concurrency | Single process | `pg_advisory_xact_lock` held while migrating |
| Backup | `notes.db.bak-v<from>-<timestamp>` written with `VACUUM INTO` before upgrading an existing database | none (use your Postgres backups) |
| Database newer than the binary | Refused with `ErrSchemaTooNew` | same |

Databases created before migrations existed are recognised as version 1
without changes. After migration 2, `ScrubSensitiveSummaries`
(`summary.go`) checks the old sensitive summaries once and records each check
in `summary_checked`.

## 10. MCP server

`inode mcp` (`cmd/mcp.go`, `internal/mcp/`) serves the Model Context Protocol
over stdio with [mcp-go](https://github.com/mark3labs/mcp-go). All three tools
are marked read-only and idempotent, so clients do not ask for approval on
every call.

| Tool | Does | Sensitive notes (default) |
|---|---|---|
| `search_notes` | Retrieval only (no LLM answer); returns the matching notes for the agent to read | content masked |
| `list_notes` | Lists by category or tag | left out entirely |
| `get_note` | One note by ID | content masked, summary withheld |

`mcp.reveal_sensitive = true` turns the masking off for all three.

## 11. Configuration

`internal/config/settings.go`. Sources, highest priority first:

1. Environment variables: `INODE_` plus the key with `.` replaced by `_`
   (`INODE_LLM_API_KEY`, `INODE_DB_DSN`, `INODE_DB_PATH`, ...).
2. A `.env` file in the current directory (loaded into the environment; does
   not override variables already set).
3. `~/.inode/config.toml`, written by `inode config set`.
4. Defaults:

| Key | Default |
|---|---|
| `llm.backend`, `embedding.backend` | `ollama`, `ollama` |
| `db.backend`, `db.path`, `db.dsn` | `sqlite`, `~/.inode/notes.db`, empty |
| `defaults.sensitive` | `true` |
| `search.max_distance`, `search.top_k` | `1.0`, `5` |
| `mcp.reveal_sensitive` | `false` |
| `log.level` | `info` |

Every key is listed once in `config.Keys` (`internal/config/keys.go`), with
its type, allowed values and whether it is a secret; a test fails if it and
`Config` disagree. `inode config set` accepts exactly those keys, checks the
value's type (`search.top_k abc` and `db.backend mysql` are rejected) and
writes typed values (`top_k = 10`). `inode config show` prints every key,
with `llm.api_key`, `embedding.api_key` and `db.dsn` redacted.

`inode config set` writes through `config.Set` (`internal/config/write.go`),
which keeps the file at mode 0600 because it can hold API keys in plaintext;
`inode config show` warns if the file is readable by others.

Model, base URL and embedding dimension have no global default. When unset,
`core.ResolveBackends` (`internal/core/backends.go`) fills them from the
selected backend; the values come from constants in the adapter packages:

| Backend | Model | Base URL | Dimension |
|---|---|---|---|
| `llm.backend = ollama` | `llama3.2` | `http://localhost:11434` | |
| `llm.backend = openai` | `gpt-4o-mini` | `https://api.openai.com/v1` | |
| `llm.backend = claude-api` | `claude-sonnet-5-5` | (SDK default) | |
| `embedding.backend = ollama` | `nomic-embed-text` | `http://localhost:11434` | 768 |
| `embedding.backend = voyage` | `voyage-3` | (fixed API URL) | 1024 |

Any other embedding model needs `embedding.dimension`; without it inode stops
instead of guessing, because the dimension sizes the vector table. `inode
config show` prints the resolved values, marked `(default for <backend>)`.
Before #31 the Ollama values were global defaults, so cloud backends inherited
them. The full reference is #74.

## 12. Files on disk

`~/.inode/`, created with mode 0700:

| File | Purpose |
|---|---|
| `config.toml` | Settings, including API keys if you set them; mode 0600 |
| `notes.db` (+ `-wal`, `-shm`) | SQLite database |
| `notes.db.bak-v<n>-<timestamp>` | Backup made before a schema upgrade |
| `.salt` | Argon2id salt |
| `.key` | Master secret, only on machines without a keychain |

## 13. Dependencies

Direct dependencies (`go.mod`): `spf13/cobra` (CLI), `spf13/viper` and
`joho/godotenv` (config), `mattn/go-sqlite3` and `asg017/sqlite-vec-go-bindings`
(SQLite, CGO), `jackc/pgx/v5` and `pgvector/pgvector-go` (Postgres),
`anthropics/anthropic-sdk-go` (Claude), `mark3labs/mcp-go` (MCP),
`zalando/go-keyring` (keychain), `golang.org/x/crypto` (Argon2id),
`google/uuid`, `fatih/color` and `mattn/go-isatty` (output).
The SQLite backend needs CGO and a C compiler; see `CONTRIBUTING.md`.

## What's next

Teams and organizations (shared notes with owner, team and mode bits, a
self-hosted server, scoped search) are designed in the open: the proposal
and its spikes are tracked in #35 and the Phase milestones on the
[board](https://github.com/orgs/BiSemaphore/projects/1). Nothing in this
document describes that work; it will be added here as it ships.
