# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- OpenAI LLM backend, opt-in via `llm.backend = openai`. Plain `net/http` against the Chat Completions API (no SDK dependency), JSON mode for classification and answers. Defaults to `gpt-4o-mini`; `llm.base_url` targets Azure OpenAI or any compatible endpoint. Ollama stays the default.

### Fixed
- Summaries of sensitive notes no longer expose the secret. They were stored and shown in plaintext in `inode list`, `inode get` sources, `inode note get` and MCP; when the LLM was unavailable the summary was the first 80 characters of the secret itself. New summaries are checked against the note and replaced with a neutral `<category> (sensitive): <tags>` when they contain secret-looking values; summaries stored by earlier versions are checked once on first run and fixed the same way. MCP `list_notes` now leaves out sensitive notes unless `mcp.reveal_sensitive` is set, and `get_note` no longer returns their summary. ([#53](https://github.com/BiSemaphore/inode/issues/53))

### Changed
- The README opens with a terminal recording of `inode add` and `inode get` instead of the old banner, and its first example now matches what the CLI prints. A social preview image for the repository is in `docs/assets/social-preview.png`. ([#110](https://github.com/BiSemaphore/inode/issues/110))
- Databases now carry a schema version and are upgraded automatically when a new inode opens them. Before upgrading a SQLite database, inode writes a backup next to it (`notes.db.bak-v<from>-<timestamp>`). A database upgraded by a newer inode is refused instead of being opened. ([#85](https://github.com/BiSemaphore/inode/issues/85))
- The repository moved to the BiSemaphore organization. The Go module path is now `github.com/BiSemaphore/inode`: install with `go install github.com/BiSemaphore/inode@latest`. Versions up to v0.4.0 remain installable under the old `github.com/shahid-io/inode` path.
- Go 1.26 or newer is required to build or `go install` inode (was 1.25, which no longer receives security fixes). ([#98](https://github.com/BiSemaphore/inode/issues/98))

### Security
- inode no longer creates a new master secret when the OS keychain cannot be read. Previously any keychain error (locked keychain, denied prompt, unavailable service) made inode generate and store a new secret, which could overwrite the existing one and make every encrypted note unreadable. A new secret is now created only on a genuine first run; otherwise inode stops with an explanation and writes nothing. ([#104](https://github.com/BiSemaphore/inode/issues/104))
- `SECURITY.md` now describes what the code actually protects and lists known limitations, including that cloud backends (`claude-api`, `openai`, `voyage`) receive note content in plaintext. Vulnerabilities are reported through GitHub private vulnerability reporting. ([#71](https://github.com/BiSemaphore/inode/issues/71))
- Release binaries are built with Go 1.26 and `golang.org/x/text` v0.43.0, fixing reachable vulnerabilities in `crypto/tls`, `net/url` and other standard-library packages, and in `golang.org/x/text`. ([#98](https://github.com/BiSemaphore/inode/issues/98))
- `golang.org/x/crypto` updated to v0.52.0, which fixes 13 advisories in its SSH and SSH-agent packages. inode uses only `x/crypto/argon2` (key derivation), so none of them were reachable, but release binaries no longer ship the affected versions. ([#102](https://github.com/BiSemaphore/inode/pull/102))

---

## [0.4.0] - 2026-06-14

### Added
- Read-only MCP server: `inode mcp` serves `search_notes`, `list_notes` and `get_note` over stdio for Claude Code, Cursor and other MCP clients.
- Sensitive notes are excluded from search results and masked in `get_note` unless `mcp.reveal_sensitive` is set.

### Fixed
- MCP tools now advertise themselves as read-only and idempotent. The SDK defaults marked them destructive, so clients asked for approval on every lookup.

---

## [0.3.0] - 2026-05-13

### Added
- Postgres + pgvector backend, opt-in via `db.backend = postgres`. Pure-Go `pgx` driver, so `go install` works without a C toolchain when this backend is selected. Same L2 distance metric (`<->` operator) as the SQLite path — the relevance threshold ports over unchanged.
- `db.dsn` config key for the Postgres connection string. `INODE_DB_BACKEND` and `INODE_DB_DSN` environment variables.
- `pgvector` service in `docker-compose.yml` alongside Ollama — one `docker compose up -d` brings both up.
- README section documenting the optional Postgres path.

### Fixed
- pgvector type registration race: `CREATE EXTENSION IF NOT EXISTS vector` now runs inside `AfterConnect` so every new pool connection bootstraps the extension before `pgxvec.RegisterTypes` looks up the `vector` type. Previously the first connect failed with `vector type not found in the database`.

---

## [0.2.1] - 2026-05-10

### Fixed
- Release workflow simplified to a single Ubuntu runner; dropped arm64 to unblock the first binary release.

### Added
- First pre-built binaries on the GitHub Releases page (`linux/amd64`).

---

## [0.2.0] - 2026-05-10

### Added
- `inode get` as the primary retrieval command (`ask`, `find`, `search` are aliases).
- Braille-frame spinner on stderr, `NO_COLOR`-aware so piped and CI runs stay plain.
- Coloured CLI output for sources, categories, tags, and the sensitive marker.
- Four additional strict categories — taxonomy went from 5 → 9 (adds `runbooks`, `references`, `contacts`, `learnings`).
- Integration test coverage for the DB, embedding, and LLM adapter layers (real SQLite + `httptest` mocks).
- Windows CI job via MSYS2 + mingw-w64 sqlite3 so the Windows build is exercised on every PR.
- README hero banner and standalone logo SVG.
- Branch protection on `main`: PR required, all status checks required, force-push blocked.

### Changed
- LLM classifications outside the strict category set now fall back to `notes` rather than being accepted as-is.
- Release workflow aligned with the rest of CI (Go 1.25, Windows MSYS2 shell).

### Fixed
- Single-note answer prompt was citing the source twice; tightened to one cite per used note.
- Empty `Sources` block no longer prints after the LLM rejects every candidate.

---

## [0.1.0] - 2026-05-08

### Added
- `inode add` — save notes, secrets, commands, and decisions with auto-classification
- `inode ask` — semantic search with RAG answer via local LLM
- `inode list` — tabular note listing with category, tag, and sensitivity filters
- `inode note get/delete` — fetch or remove a note by ID prefix
- `inode config set/show` — manage configuration without editing files
- Local-first backend: Ollama (llama3.2 + nomic-embed-text) — no API keys required
- Cloud backend support: Claude API + Voyage AI (opt-in via config)
- AES-256-GCM encryption for sensitive notes with Argon2id key derivation
- SQLite + sqlite-vec for local vector similarity search
- Docker Compose setup for Ollama

[Unreleased]: https://github.com/BiSemaphore/inode/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/BiSemaphore/inode/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/BiSemaphore/inode/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/BiSemaphore/inode/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/BiSemaphore/inode/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/BiSemaphore/inode/releases/tag/v0.1.0
