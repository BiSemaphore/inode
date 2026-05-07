# inode

> Personal knowledge base and secret vault — semantic search via vector embeddings and RAG.

**inode** is a CLI tool for storing notes, secrets, API keys, commands, and decisions — retrieved via vector similarity search and LLM inference instead of grep, folders, or memory.

```bash
$ inode add "My Stripe test key is sk_test_xxxxx"
  ✓ category=credentials  tags=[stripe, payment, test]  sensitive=true
  ✓ Saved

$ inode ask "stripe test key"
  Note — "Stripe test secret key" [credentials]
  Value: sk_test_******  (use --reveal to show)
```

---

## Features

- **Natural language search** — ask in plain English, get the right note back
- **Auto-tagging** — Claude automatically categorizes and tags everything you save
- **Sensitive value protection** — secrets are AES-256-GCM encrypted at rest, masked by default
- **Zero infra** — local SQLite, no server, no account needed for Phase 1
- **Cross-platform** — single binary for macOS, Linux, and Windows

---

## Install

### macOS (Homebrew)

```bash
brew install shahidraza/tap/inode
```

### Linux / Windows

Download the latest binary from [GitHub Releases](https://github.com/shahid-io/inode/releases).

### Build from source

```bash
go install github.com/shahid-io/inode@latest
```

---

## Quick Start

```bash
# 1. Set your API keys
inode config set llm.api_key sk-ant-xxxx
inode config set embedding.api_key pa-xxxx

# 2. Save something
inode add "My GitHub PAT is ghp_xxxxxxxxxx"

# 3. Find it later
inode ask "github personal access token"
```

---

## Commands

```bash
# Add notes
inode add "note content"
inode add "secret" --sensitive
inode add "docker system prune -a" --category commands --tags docker,cleanup
inode add                          # opens $EDITOR

# Search
inode ask "query"
inode ask "query" --reveal         # unmask sensitive values

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

---

## Documentation

- [`docs/spec.md`](docs/spec.md) — full product specification
- [`docs/architecture.md`](docs/architecture.md) — technical architecture

---

## Roadmap

| Phase | Status | Description |
|---|---|---|
| Phase 1 — Local MVP | 🚧 In progress | CLI, SQLite, Claude API, encryption |
| Phase 2 — Cloud | Planned | Multi-user, PostgreSQL, JWT auth |
| Phase 3 — LLM Swappability | Planned | Ollama local model support |
| Phase 4 — Hardening | Planned | 2FA, rate limiting, audit log |
| Phase 5 — Ecosystem | Planned | Web dashboard, MCP server, team workspaces |

---

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.

---

## Security

inode handles secrets and sensitive data. If you discover a vulnerability, please follow responsible disclosure — see [SECURITY.md](SECURITY.md).

---

## License

[MIT](LICENSE) © 2026 Shahid Raza
