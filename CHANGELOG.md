# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/shahid-io/inode/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/shahid-io/inode/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/shahid-io/inode/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/shahid-io/inode/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/shahid-io/inode/releases/tag/v0.1.0
