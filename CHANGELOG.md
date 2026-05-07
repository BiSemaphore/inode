# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/shahid-io/inode/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/shahid-io/inode/releases/tag/v0.1.0
