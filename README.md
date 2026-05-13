<div align="center">
  <img src="docs/banner.svg" alt="inode — a personal knowledge base that runs on your laptop" width="100%">
</div>

<br/>

**inode** is a CLI tool for storing notes, secrets, API keys, commands, and decisions — retrieved via vector similarity search and LLM inference instead of grep, folders, or memory. It runs entirely on your machine; the default backends are local (Ollama for both LLM and embeddings, SQLite for storage).

```bash
$ inode add "My Stripe test key is sk_test_xxxxx"
  ✓ category=credentials  tags=[stripe, payment, test]  sensitive=true
  ✓ Saved

$ inode get "stripe test key"
  Note — "Stripe test secret key" [credentials]
  Value: sk_test_******  (use --reveal to show)
```

---

## Features

- **Natural-language retrieval** — ask in plain English, get the right note back
- **Auto-classification** — the LLM picks the category and tags from a curated set
- **Sensitive-value protection** — secrets are AES-256-GCM encrypted at rest, masked by default
- **Local-first by default** — Ollama for LLM and embeddings, SQLite for storage; no API keys, no account, no internet required
- **Cross-platform** — single Go binary on macOS, Linux, and Windows
- **Optional cloud backends** — Claude API + Voyage AI for higher-quality answers if you want them

---

## Install

### Build from source (recommended while we iterate)

```bash
go install github.com/shahid-io/inode@latest
```

### Releases

Pre-built binaries for macOS / Linux / Windows are published on [GitHub Releases](https://github.com/shahid-io/inode/releases).

---

## Quick Start (zero-cost path)

```bash
# 1. Install Ollama and pull the local models (one-time, free)
brew install ollama
ollama pull llama3.2
ollama pull nomic-embed-text

# 2. Save something
inode add "My GitHub PAT is ghp_xxxxxxxxxx"

# 3. Find it later
inode get "github personal access token"
```

That's it. No API keys, no account, no telemetry. Everything stays on your laptop.

### Optional: cloud backends

If you want Claude-quality answers and Voyage-quality embeddings:

```bash
inode config set llm.backend claude-api
inode config set llm.api_key sk-ant-xxxx
inode config set embedding.backend voyage
inode config set embedding.api_key pa-xxxx
```

These are completely optional. The Ollama path is fully featured.

### Optional: Postgres + pgvector backend

By default inode uses SQLite + sqlite-vec — single file, zero infrastructure.

If you're on Windows (where the sqlite-vec CGO build chain is painful) or you'd rather run on a real database, you can switch to Postgres + pgvector. The Postgres driver is pure Go, so `go install` works without any C toolchain.

```bash
# 1. Start a pgvector-enabled Postgres (compose file in repo root)
docker compose up -d

# 2. Point inode at it
inode config set db.backend postgres
inode config set db.dsn "postgres://postgres:password@localhost:5432/postgres?sslmode=disable"
```

inode will `CREATE EXTENSION vector` and create the `notes` table on first run. SQLite remains the default — Postgres is opt-in.

### Optional: MCP server (Claude Code / Cursor)

`inode mcp` runs as a Model Context Protocol server over stdio so an MCP-aware AI client can read your knowledge base.

Exposes three read-only tools to the calling agent:

- `search_notes` — vector search; returns the most relevant notes
- `list_notes` — paginated listing; metadata only
- `get_note` — fetch a single note by ID prefix

Sensitive notes are excluded from search results and masked in `get_note` responses by default. To let the agent see them:

```bash
inode config set mcp.reveal_sensitive true
```

Wire it into your MCP client config (Claude Code example):

```json
{
  "mcpServers": {
    "inode": { "command": "inode", "args": ["mcp"] }
  }
}
```

Writing notes from an agent isn't exposed yet — read-only is the safer first step.

---

## Commands

```bash
# Add notes
inode add "note content"
inode add "secret" --sensitive
inode add "docker system prune -a" --category commands --tags docker,cleanup
inode add                          # opens $EDITOR

# Retrieve (aliases: ask, find, search)
inode get "query"
inode get "query" --reveal         # unmask sensitive values

# List
inode list
inode list --category credentials
inode list --tag github

# Manage
inode note get <id>
inode note edit <id>
inode note delete <id>

# Config
inode config set llm.backend claude-api
inode config set llm.model claude-sonnet-4-6
inode config show
```

### Categories

inode classifies every note into one of nine strict categories:

`credentials` · `commands` · `snippets` · `decisions` · `runbooks` · `learnings` · `references` · `contacts` · `notes`

If the LLM proposes something else, the classifier falls back to `notes`. You can override with `--category` and the classifier won't second-guess you.

---

## Documentation

- [`docs/spec.md`](docs/spec.md) — full product specification
- [`docs/architecture.md`](docs/architecture.md) — technical architecture
- [`docs/logo.svg`](docs/logo.svg) — standalone logo for forks / social

---

## Roadmap

| Phase | Status | Description |
|---|---|---|
| Phase 1 — Local MVP | **Shipped** | CLI, SQLite + sqlite-vec, encryption, semantic retrieval |
| Phase 3 — Local LLM | **Shipped** | Ollama default for LLM and embeddings |
| Phase 2 — Cloud | Hypothetical | Multi-user, PostgreSQL, JWT auth |
| Phase 4 — Hardening | Planned | 2FA, rate limiting, audit log |
| Phase 5 — Ecosystem | Planned | Web dashboard, MCP server, team workspaces |

The original phase numbering had Cloud at Phase 2 and Local LLM at Phase 3. In practice, Local LLM shipped first because it's also the architectural posture — local-first is the design, not a feature behind a future-Phase fence.

---

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.

---

## Security

inode handles secrets and sensitive data. If you discover a vulnerability, please follow responsible disclosure — see [SECURITY.md](SECURITY.md).

---

## License

[MIT](LICENSE) © 2026 Shahid Raza
