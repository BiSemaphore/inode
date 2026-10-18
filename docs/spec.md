# inode product spec

What inode is, who it is for, and what it does and does not do, as of v0.5.0.
How it is built is in [architecture.md](architecture.md).

## What it is

A knowledge base you use from the terminal. You save anything in one line (a
note, a command, a decision, an API key) and later ask for it in plain
English. inode classifies and tags what you save, encrypts what is sensitive,
and answers questions from your notes with the notes it used listed as
sources.

It runs on your machine by default: Ollama for the language model and
embeddings, SQLite for storage, no account and no network service. Cloud
backends and Postgres are opt-in.

## Who it is for

Developers who keep reusable knowledge scattered across shell history, chat
threads, notes apps and `.env` files, and who would rather ask than search.
Today inode is single-user: one person, one knowledge base. Sharing with a
team is the next phase (see [What's next](#whats-next)).

## Problems it solves

| You want | Without inode | With inode |
|---|---|---|
| The command you ran three weeks ago | `Ctrl+R`, grep history, search the docs again | `inode get "rotate staging db creds"` |
| An API key for a service you rarely use | Check five places | `inode get "stripe test key"` |
| Why a decision was made | Read old threads | `inode get "why did we pick NATS"` |
| A runbook step during an incident | Find the wiki page | `inode get "rollback the api deploy"` |

## Features (shipped)

| Feature | Since |
|---|---|
| `add`, `list`, `note get/delete`, `config`, and search as `ask` | v0.1.0 |
| Search renamed to `get`, with `ask` kept as an alias | v0.2.0 |
| Automatic category, tags, sensitivity and one-line summary | v0.1.0 |
| AES-256-GCM encryption for sensitive notes, master secret in the OS keychain | v0.1.0 |
| Local default: Ollama (LLM and embeddings) and SQLite with sqlite-vec | v0.1.0 |
| Postgres with pgvector as an alternative store | v0.3.0 |
| MCP server (`inode mcp`), read-only, for Claude Code, Cursor and other MCP clients | v0.4.0 |
| OpenAI LLM backend | v0.5.0 |
| Versioned schema migrations with an automatic backup | v0.5.0 |
| Secrets kept out of summaries; an existing master secret is never replaced | v0.5.0 |

## Commands

| Command | Does |
|---|---|
| `inode add "<text>"` | Save a note. Flags: `--category`, `--tags a,b`, `--sensitive`, `--no-sensitive` |
| `inode get "<question>"` | Answer from your notes, with sources. Aliases: `ask`, `find`, `search`. Flag: `--reveal` (asks first) |
| `inode list` | List notes. Flags: `--category`, `--tag`, `--limit` (default 20), `--sensitive` (asks first) |
| `inode note get <id>` | Show one note; asks before revealing sensitive content |
| `inode note delete <id>` | Delete one note |
| `inode config set <key> <value>`, `inode config show` | Change or print settings; `set` checks the key and value type, `show` prints every key with API keys and `db.dsn` redacted |
| `inode mcp` | Run the MCP server over stdio |
| `inode version` | Print the version |

IDs can be shortened to a prefix; the CLI prints the first 8 characters.

## Categories

The language model must pick one of these; anything else becomes `notes`
(`internal/model/note.go`).

| Category | For |
|---|---|
| `credentials` | API keys, tokens, passwords, secrets |
| `commands` | CLI commands and one-liners you reuse |
| `snippets` | Code, config, queries, small reusable text |
| `decisions` | Technical and process decisions, with the reasoning |
| `runbooks` | Multi-step procedures, deploy and incident playbooks |
| `learnings` | Gotchas, debugging insights, lessons from incidents |
| `references` | URLs, docs, dashboards |
| `contacts` | People, emails, handles, on-call rotations |
| `notes` | Everything else |

## Sensitive notes

Notes are sensitive by default (`defaults.sensitive = true`); `--no-sensitive`
or the config turns that off. A sensitive note's content is encrypted at rest
and masked in every output until you confirm a reveal. Its summary, category,
tags and embedding are not encrypted. The summary is checked so it never
repeats a secret. With a cloud backend, content is sent to that provider to be
classified, embedded and answered. Details: [architecture.md](architecture.md)
sections 6 and 7, and [SECURITY.md](../SECURITY.md).

## Backends

| Role | Default (runs on your machine, free) | Optional |
|---|---|---|
| Language model | Ollama, `llama3.2` | Anthropic Claude (`claude-api`), OpenAI (`openai`) |
| Embeddings | Ollama, `nomic-embed-text` | Voyage AI (`voyage`) |
| Storage | SQLite + sqlite-vec, `~/.inode/notes.db` | Postgres + pgvector (`db.backend = postgres`) |

## Platforms

Release archives are built for linux/amd64. On macOS, Windows and other
architectures, `go install github.com/BiSemaphore/inode@latest` works where CGO
and a C compiler are available (the SQLite backend needs them). More release
targets are tracked in #33.

## Not goals

- **A hosted service.** inode runs on your machine, or on a server you run.
  There is no inode cloud and no paid tier.
- **Sync between devices.** Several machines can point at one Postgres
  database, but sensitive notes are encrypted with the master secret of the
  machine that saved them, so they only open there.
- **A replacement for a team secrets manager** such as Vault. inode keeps your
  own working knowledge, secrets included, close at hand; it does not issue,
  rotate or audit credentials.
- **Editing notes in place.** Delete and add again.
- **Full-text or keyword search.** Retrieval is semantic, by meaning.

## What's next

Teams and organizations: notes shared like files, with an owner, a team and a
mode (`chmod 640`), a self-hosted server, search that only sees what you may
read, and agents that search as the person running them. The design is being
worked out in public in #35 and the spikes linked from it, and the order of work
is on the [board](https://github.com/orgs/BiSemaphore/projects/1) under the
Phase milestones. This document will describe it once it ships.
