# inode — Product Specification

> A developer-first, AI-powered personal knowledge base and secret vault.
> Save anything once. Find it later with natural language.

---

## 1. What is inode?

inode is a CLI-first tool for storing notes, secrets, keys, commands, and decisions — and
retrieving them later using natural language instead of folder hierarchies or keyword search.

**Core promise:** You saved it once. You never need to remember where. Just ask.

**Primary user:** Individual developers, personal productivity.
**Future scope:** Teams and organizations.

---

## 2. Problems It Solves

| Scenario | Without inode | With inode |
|---|---|---|
| Find a GitHub PAT saved 3 months ago | Scroll Notion, notes app, emails | `inode ask "github personal access token"` |
| Recall a complex bash command | CTRL+R, grep history, search docs | `inode ask "docker cleanup command"` |  
| Find an API key for a rarely-used service | Check 5 different places | `inode ask "stripe test key"` |
| Get context on a past decision | Read through meeting notes | `inode ask "why did we choose postgres"` |

---

## 3. Feature Set

### Phase 1 — Local MVP

- [ ] `inode add` — save a note (text, secret, snippet, command, anything)
- [ ] `inode ask` — natural language search using RAG over saved notes
- [ ] `inode list` — list notes with filters (category, tag, sensitive)
- [ ] `inode config` — manage local config (LLM backend, API keys, defaults)
- [ ] Auto-tagging and auto-categorization via Claude API
- [ ] Sensitive note flagging (`--sensitive` / `--no-sensitive` flags)
- [ ] Sensitive value masking in output; `--reveal` flag to show full value
- [ ] AES-256-GCM encryption at rest for sensitive note content
- [ ] Semantic embeddings via Voyage AI for natural language search

### Phase 2 — Multi-user + Cloud

- [ ] Go + chi HTTP server with REST API
- [ ] PostgreSQL + pgvector (replaces SQLite + sqlite-vec)
- [ ] User accounts, JWT auth, refresh tokens
- [ ] Browser-based login flow (`inode login`)
- [ ] Per-device session tokens, revocable from dashboard

### Phase 3 — LLM Swappability

- [ ] Ollama integration (local LLM, no API cost, offline)
- [ ] Config-driven backend switching
- [ ] LLM adapter pattern validated across Claude + Ollama

### Phase 4 — Hardening

- [ ] Rate limiting
- [ ] 2FA (TOTP — Google Authenticator)
- [ ] CLI shell autocomplete
- [ ] Audit log for sensitive value reveals

### Phase 5 — Scale and Ecosystem

- [ ] Web dashboard (Next.js)
- [ ] Bulk import (Notion, markdown files)
- [ ] Note sharing between users
- [ ] Team and org workspaces
- [ ] MCP server — expose inode as a Claude Desktop tool
- [ ] Scheduled reminders

---

## 4. Data Model

### Note

```
id:            UUID
user_id:       UUID (FK → users; null in Phase 1 local mode)
content_enc:   BLOB   (AES-256-GCM encrypted; stores ciphertext + IV + auth tag)
content_plain: TEXT   (null when is_sensitive=true, plaintext otherwise)
summary:       TEXT   (LLM-generated one-liner for list display)
category:      STRING ("credentials" | "commands" | "decisions" | "references" | "notes")
tags:          JSON   (array of strings, e.g. ["github", "token", "personal"])
is_sensitive:  BOOLEAN
embedding:     BLOB   (serialized float32 array; 1024 dims for voyage-3)
created_at:    TIMESTAMP
updated_at:    TIMESTAMP
```

### User (Phase 2+)

```
id:            UUID
email:         STRING (unique)
username:      STRING (unique)
password_hash: STRING (Argon2id)
created_at:    TIMESTAMP
last_login:    TIMESTAMP
```

### Config (Phase 2+, server-side per user)

```
user_id:               UUID
llm_backend:           ENUM ("claude-api" | "ollama" | "openai")
llm_model:             STRING
api_key_enc:           STRING (encrypted)
embedding_backend:     ENUM ("voyage" | "local")
default_sensitive:     BOOLEAN
predefined_tags:       JSON array
predefined_categories: JSON array
```

---

## 5. Predefined Categories

```json
[
  { "name": "credentials", "description": "API keys, tokens, passwords, secrets, access keys" },
  { "name": "commands",    "description": "CLI commands, bash scripts, terminal shortcuts" },
  { "name": "decisions",   "description": "Architectural decisions, tech choices, meeting outcomes" },
  { "name": "references",  "description": "URLs, documentation links, resource pointers" },
  { "name": "notes",       "description": "General notes, todos, reminders, ideas" }
]
```

The LLM can suggest new categories when no predefined one fits. Custom categories persist
to config and are included in future classification prompts.

---

## 6. Auto-Tagging and Categorization

When a note is added without explicit tags or category, inode sends the content to Claude
with the predefined category list and tag suggestions. Claude returns structured JSON:

```json
{
  "category": "credentials",
  "tags": ["stripe", "payment", "test"],
  "is_sensitive": true,
  "summary": "Stripe test secret key"
}
```

When the user specifies tags or category manually, they are validated against the predefined
list. Custom values that don't match are passed to Claude for fuzzy reconciliation — Claude
maps them to the nearest predefined option or confirms a new one.

---

## 7. CLI Reference

### Authentication (Phase 2+)

```bash
inode login       # Opens browser for OAuth or prompts email/password
inode logout
inode whoami
```

### Adding Notes

```bash
inode add "My GitHub PAT is ghp_xxx"
inode add "docker system prune -a" --category commands --tags docker,cleanup
inode add --sensitive "AWS_SECRET=xxxx"    # force sensitive
inode add --no-sensitive "Just a tip"      # force not sensitive
inode add                                   # opens $EDITOR (like git commit)
```

### Searching

```bash
inode ask "what is my github token"
inode ask "show me docker cleanup commands"
inode ask "any AWS credentials I saved"
inode ask "stripe test key" --reveal       # unmask sensitive values
```

### Listing

```bash
inode list                          # all notes, sensitive values masked
inode list --category credentials
inode list --tag github
inode list --sensitive              # requires confirmation prompt
```

### Note Management

```bash
inode note get <id>
inode note edit <id>
inode note delete <id>
```

### Config

```bash
inode config set llm.backend claude-api
inode config set llm.model claude-sonnet-4-6
inode config set llm.api_key sk-ant-xxxx
inode config set embedding.backend voyage
inode config set embedding.api_key pa-xxxx
inode config set defaults.sensitive true
inode config show
```

---

## 8. Example Session

```bash
$ inode add "My Stripe test secret key is sk_test_xxxxx"
  Auto-detected: category=credentials, tags=[stripe, payment, test]
  Flagged as: sensitive=true
  Saved as note #47

$ inode ask "stripe test key"
  Note #47 — "Stripe test secret key" [credentials] [stripe, payment, test]
  Value: sk_test_****** (use --reveal to show)
  Saved: 2 days ago

$ inode ask "stripe test key" --reveal
  Reveal sensitive value? [y/N]: y
  sk_test_xxxxx

$ inode list --category credentials
  #47  Stripe test secret key     [stripe, payment, test]   2 days ago   ••••••
  #31  GitHub personal token      [github, token]           3 weeks ago  ••••••
  #12  AWS production access key  [aws, prod]               1 month ago  ••••••
```

---

## 9. Security Model

| Layer | Implementation |
|---|---|
| Sensitive content at rest | AES-256-GCM; key derived via Argon2id, stored in OS keychain |
| Reveal flow | Explicit `--reveal` flag + confirmation prompt required |
| Embeddings | Semantic-only vectors; raw content cannot be reconstructed from them |
| Config API keys | Encrypted with same keychain-derived key |
| Phase 2: passwords | Argon2id hashing (no bcrypt — Argon2id is stronger) |
| Phase 2: sessions | JWT (1h expiry) + refresh tokens (30d), per-device, revocable |
| Phase 2: transport | HTTPS only |
| Phase 2: 2FA | TOTP (Google Authenticator / Authy) |

### Encryption Key Derivation

```
secret = OS keychain secret (random 32 bytes, generated on first run)
salt   = fixed per-install salt (stored in ~/.inode/config.toml)
key    = Argon2id(secret, salt, len=32)

Per note:
  iv         = random 12 bytes (new per note)
  ciphertext, auth_tag = AES_256_GCM(key, iv, plaintext)

Stored in DB: { ciphertext, iv, auth_tag }
```

---

## 10. Local Config File

Stored at `~/.inode/config.toml` (file permissions: 600).

```toml
[llm]
backend     = "claude-api"
model       = "claude-sonnet-4-6"
api_key_enc = "enc:base64encodedciphertext..."   # AES-256-GCM encrypted

[embedding]
backend     = "voyage"
model       = "voyage-3"
api_key_enc = "enc:base64encodedciphertext..."

[db]
path = "~/.inode/notes.db"   # Phase 1 (SQLite + sqlite-vec)
# url = "postgresql://..."   # Phase 2

[security]
salt = "base64randomsalt..."  # generated on first run, never changes

[defaults]
sensitive = true              # new notes flagged sensitive by default
```

---

## 11. Tech Stack

### Phase 1 — Local MVP

| Layer | Choice | Reason |
|---|---|---|
| Language | Go | Single binary, cross-platform (Windows/Mac/Linux), zero runtime dependency |
| CLI framework | Cobra | Industry standard Go CLI framework (used by kubectl, gh, docker) |
| Terminal output | Lipgloss + Bubbles | Rich styling, tables, spinners — Charm ecosystem |
| LLM | `anthropic-sdk-go` (official) | Anthropic-maintained Go SDK |
| Embeddings | Voyage AI — `voyage-3` | Anthropic's recommended embedding partner; custom HTTP client (~50 lines) |
| Vector search | SQLite + `sqlite-vec-go-bindings` | Zero infra; cosine similarity on a laptop |
| Encryption | Go stdlib `crypto/aes` + `crypto/cipher` | AES-256-GCM, no third-party lib needed |
| Key storage | `go-keyring` | macOS Keychain, Linux Secret Service, Windows Credential Locker |
| Config | `BurntSushi/toml` | TOML parsing for `~/.inode/config.toml` |

### Phase 2+ — Cloud

| Layer | Choice |
|---|---|
| API Server | Go + `chi` router (same language, no context switch) |
| Database | PostgreSQL + pgvector |
| Auth | JWT (`golang-jwt/jwt`) + Argon2id (`golang.org/x/crypto/argon2`) |
| Hosting | Railway or Fly.io |
| Frontend | Next.js |

### Distribution

| Platform | Method |
|---|---|
| macOS | `brew install inode` or download binary |
| Linux | Download binary or `apt`/`yum` package |
| Windows | `winget install inode` or download `.exe` |
| All | GitHub Releases (cross-compiled from Mac via `goreleaser`) |

---

## 12. Development Phases

### Phase 1 — Local MVP (target: 1–2 weeks)

Build `inode add`, `inode ask`, `inode list`, `inode config`. Everything runs locally.
SQLite + sqlite-vec. Claude API + Voyage AI. AES-256-GCM for sensitive notes.
No server, no auth, single user.

### Phase 2 — Multi-user + Cloud (target: 2–3 weeks)

Introduce Go + chi HTTP server, PostgreSQL + pgvector, JWT auth, and the `inode login` flow.
The CLI switches from calling local services to HTTP. Core business logic is unchanged.

### Phase 3 — LLM Swappability (target: 1 week)

Ollama integration behind the existing LLM adapter. Config-driven switching.
Validate that all LLM operations work identically across Claude and Ollama.

### Phase 4 — Hardening (ongoing)

Rate limiting, 2FA, audit logging for sensitive reveals, CLI autocomplete.

### Phase 5 — Scale and Ecosystem

Web dashboard, MCP server, team workspaces, bulk import, scheduled reminders.
