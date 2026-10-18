<div align="center">
  <img src="docs/assets/demo.gif" alt="Terminal recording: inode add saves a note about rotating staging database credentials and marks it sensitive; inode get answers a plain-English question and lists the note as its source" width="100%">
</div>

<br/>

[![CI](https://github.com/BiSemaphore/inode/actions/workflows/ci.yml/badge.svg)](https://github.com/BiSemaphore/inode/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/BiSemaphore/inode)](https://github.com/BiSemaphore/inode/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/BiSemaphore/inode)](go.mod)
[![License](https://img.shields.io/github/license/BiSemaphore/inode)](LICENSE)

**inode** is a command-line knowledge base for the things engineers keep losing: commands, API keys, decisions, runbooks. You save a note in plain text and later ask for it in plain English. inode finds it by meaning (vector search) and answers from your notes only. By default it runs on your machine with Ollama and SQLite: no API keys, no account.

```console
$ inode add "staging DB creds: vault write database/rotate-root/staging"
  category=runbooks  tags=[vault, postgres, staging]  sensitive=true
  saved  id=a1b2c3d4
$ inode get "how do I rotate staging db creds"
Run vault write database/rotate-root/staging, then restart the api.

── Sources (1 note(s)) ──
  #a1b2c3d4  Rotate staging DB creds  [runbooks] [vault, postgres, staging]  [sensitive]
```

---

## What it does

- **Ask in plain English.** `inode get` turns your question into an embedding, finds the closest notes, and has the LLM answer from those notes, listing them as sources. When nothing relevant is stored, it says so.
- **Sorts notes for you.** Each note gets one of nine categories and a few tags from the LLM. `--category` and `--tags` override it.
- **Keeps secrets out of sight.** Notes marked sensitive are encrypted with AES-256-GCM before they are written and stay masked in output until you confirm a reveal. A summary that repeats a secret-looking value from the note is replaced. Details in [SECURITY.md](SECURITY.md).
- **Runs offline by default.** Ollama for the LLM and embeddings, SQLite (with sqlite-vec) for storage.
- **Other backends when you want them.** Claude or OpenAI for the LLM, Voyage for embeddings, Postgres with pgvector for storage.
- **Works with coding agents.** `inode mcp` is a read-only MCP server, so Claude Code or Cursor can search your notes.

---

## Install

```bash
go install github.com/BiSemaphore/inode@latest
inode version
```

You need **Go 1.26 or newer** and **a C compiler**: SQLite and sqlite-vec are compiled in through CGO, whichever storage backend you use.

| Platform | How |
|---|---|
| macOS | `go install`; the compiler comes with Xcode Command Line Tools (`xcode-select --install`) |
| Linux | `go install` with `gcc`, or the linux/amd64 archive on [Releases](https://github.com/BiSemaphore/inode/releases) |
| Windows | `go install` from an MSYS2 MinGW-w64 shell with `mingw-w64-x86_64-gcc` and `mingw-w64-x86_64-sqlite3`; see [CONTRIBUTING.md](CONTRIBUTING.md#prerequisites) |

CI builds and tests on all three. Release archives exist for linux/amd64 only for now; more targets are tracked in [#33](https://github.com/BiSemaphore/inode/issues/33). Versions before v0.5.0 were published as `github.com/shahid-io/inode`.

---

## Quick start

```bash
# 1. Install Ollama (https://ollama.com/download), start it, pull two models
ollama pull llama3.2           # answers, categories, summaries
ollama pull nomic-embed-text   # embeddings for search

# 2. Save something
inode add "docker system prune -af removes all unused images and containers"
inode add "stripe test key sk_test_xxxxxxxx" --sensitive

# 3. Find it later
inode get "how do I clean up docker"
inode list
```

The first command creates `~/.inode/notes.db` and a master secret in your OS keychain (used to encrypt sensitive notes). Nothing else to set up.

---

## Other backends (optional)

Every setting can go in `~/.inode/config.toml` with `inode config set <key> <value>`, or in an environment variable named `INODE_` plus the key in capitals with `.` replaced by `_` (`INODE_LLM_API_KEY`). `inode config set --help` lists every key and `inode config show` prints the values in use.

**Claude for answers, Voyage for embeddings:**

```bash
inode config set llm.backend claude-api
inode config set llm.api_key sk-ant-xxxx
inode config set embedding.backend voyage
inode config set embedding.api_key pa-xxxx
```

**OpenAI for answers** (`llm.base_url` also points it at Azure OpenAI or any compatible endpoint):

```bash
inode config set llm.backend openai
inode config set llm.api_key sk-xxxx
```

Each backend uses its own default model unless you set `llm.model` or `embedding.model`: `llama3.2`, `claude-sonnet-5-5`, `gpt-4o-mini`, `nomic-embed-text`, `voyage-3`. For any other embedding model, also set `embedding.dimension`.

Cloud backends receive the text of your notes, sensitive ones included: on `inode add` to classify and embed them, and on `inode get` as the sources for the answer. See [SECURITY.md](SECURITY.md).

Choose the embedding backend before adding notes: notes embedded by one model cannot be searched with another, and re-embedding is not built yet ([#130](https://github.com/BiSemaphore/inode/issues/130), [#131](https://github.com/BiSemaphore/inode/issues/131)).

**Postgres with pgvector** instead of SQLite:

```bash
docker compose up -d pgvector    # compose file in the repo root
inode config set db.backend postgres
inode config set db.dsn "postgres://postgres:password@localhost:5432/postgres?sslmode=disable"
```

inode creates the `vector` extension and its tables on first run. Schema upgrades run automatically on both backends; for SQLite, inode first writes a backup next to the database (`notes.db.bak-v<n>-<time>`).

---

## Use it from Claude Code or Cursor (MCP)

`inode mcp` serves three read-only tools over stdio:

| Tool | Returns |
|---|---|
| `search_notes` | the notes closest to a query |
| `list_notes` | notes by category or tag, metadata only |
| `get_note` | one note, by ID or unique ID prefix |

Sensitive notes are left out of `search_notes` and `list_notes`, and `get_note` masks their content. `inode config set mcp.reveal_sensitive true` lets the agent read them.

Claude Code:

```bash
claude mcp add inode -- inode mcp
```

Other clients (for Cursor, `~/.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "inode": { "command": "inode", "args": ["mcp"] }
  }
}
```

Adding notes from an agent is not exposed yet ([#58](https://github.com/BiSemaphore/inode/issues/58)).

---

## Commands

```bash
# Add (with no argument, opens $EDITOR)
inode add "note content"
inode add "a secret" --sensitive
inode add "brew upgrade" --no-sensitive
inode add "docker system prune -a" --category commands --tags docker,cleanup

# Ask (aliases: ask, find, search)
inode get "query"
inode get "query" --reveal              # unmask sensitive values, after a confirmation

# List
inode list                              # newest 20; --limit to change
inode list --category credentials
inode list --tag github
inode list --sensitive                  # only sensitive notes, after a confirmation

# One note (its ID, or a unique prefix of the ID shown by inode list)
inode note get <id>
inode note delete <id>                  # at least 4 characters; asks first

# Settings
inode config set <key> <value>
inode config show

# Other
inode mcp                               # MCP server over stdio
inode version
```

### Categories

`credentials`, `commands`, `snippets`, `decisions`, `runbooks`, `learnings`, `references`, `contacts`, `notes`.

If the LLM suggests anything else, the note goes to `notes`. `--category` overrides the LLM.

---

## Status and roadmap

inode is a single-user tool today, released as v0.5.x. The next phases add a self-hosted server so a team can share notes with permissions, then what an organization needs (sign-in through its identity provider, admin roles, audit). Design questions are settled in spike issues before any of it is built.

- [Milestones](https://github.com/BiSemaphore/inode/milestones): what is planned for each release and phase
- [CHANGELOG.md](CHANGELOG.md): what shipped in each version

---

## Documentation

- [`docs/spec.md`](docs/spec.md): what inode does, for whom, and what it does not do
- [`docs/architecture.md`](docs/architecture.md): how it is built, file by file, including every setting and its default
- [CONTRIBUTING.md](CONTRIBUTING.md): setup, workflow, and what a pull request needs
- [RELEASING.md](RELEASING.md): how a version is cut
- [SECURITY.md](SECURITY.md): what is protected, known limitations, and how to report a vulnerability

---

## License

[MIT](LICENSE) © 2026 Shahid Raza
