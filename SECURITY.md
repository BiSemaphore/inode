# Security Policy

inode stores secrets, API keys and other sensitive developer data, so security
reports are handled before other work.

## Reporting a vulnerability

**Do not open a public issue for a vulnerability.**

Report it privately through GitHub: open the repository's **Security** tab and
choose **Report a vulnerability**
([direct link](https://github.com/BiSemaphore/inode/security/advisories/new)).
Only the maintainers can see the report.

Please include:

- what an attacker can do, and what they need first (local access, a malicious
  config file, a compromised backend, ...)
- steps to reproduce, with fake secrets
- the inode version (`inode version`), OS, and backends in use
- a suggested fix, if you have one

### What to expect

inode is maintained by a small team. We aim to:

- acknowledge a report within **7 days**
- share an assessment and a fix or mitigation plan within **30 days** for
  confirmed issues
- credit the reporter in the advisory and release notes, unless you prefer not

Fixes ship in a new release with a GitHub security advisory and a CHANGELOG
entry under **Security**.

## Supported versions

inode is pre-1.0. Security fixes go into the **latest minor release** only; the
fix ships as a new patch or minor version. Upgrade with
`go install github.com/BiSemaphore/inode@latest` or from the
[releases page](https://github.com/BiSemaphore/inode/releases).

| Version | Supported |
|---|---|
| Latest minor release | Yes |
| Older releases | No |

## How inode protects data

| Area | What the code does | Where |
|---|---|---|
| Sensitive note content at rest | AES-256-GCM with a random 12-byte nonce per encryption; stored as `nonce + ciphertext + tag` in `content_enc`, `content_plain` left empty | `internal/core/encryption.go` (`Encrypt`, `Decrypt`), `internal/core/notes.go` (`Add`) |
| Encryption key | Derived per process with Argon2id (time 2, memory 64 MiB, 1 thread, 32-byte key) from a master secret and a 16-byte salt; never written to disk | `internal/core/encryption.go` (`DeriveKey`) |
| Master secret | 32 random bytes stored in the OS keychain (macOS Keychain, Linux Secret Service, Windows Credential Manager). If no keychain is reachable (headless Linux, WSL), it falls back to `~/.inode/.key` with mode 0600 and prints a warning. An existing secret is never replaced: if the keychain cannot be read after setup, inode stops instead of creating a new one | `internal/core/encryption.go` (`ensureSecret`, `writeKeyFile`) |
| Salt | `~/.inode/.salt`, mode 0600 | `internal/core/encryption.go` (`loadOrCreateSalt`) |
| Data directory | `~/.inode` is created with mode 0700 | `cmd/config.go`, `internal/core/encryption.go` (`NewKeyManager`) |
| Config file | `~/.inode/config.toml` is written with mode 0600; a file written by an earlier version is tightened on the next `inode config set`, and `inode config show` warns while it is not | `internal/config/write.go` |
| Output masking | Sensitive content is masked by default in `inode get`, `inode list` and `inode note get`; `--reveal`, `list --sensitive` and revealing in `note get` ask for confirmation | `internal/core/search.go` (`MaskSensitive`), `cmd/get.go`, `cmd/list.go`, `cmd/note.go` |
| MCP server | Sensitive notes are excluded from `search_notes` results and masked in `get_note` unless `mcp.reveal_sensitive` is set; all tools are read-only | `internal/mcp/tools.go`, `internal/mcp/server.go` |
| Default backends | Ollama (LLM and embeddings) and SQLite run on your machine; nothing leaves it unless you configure a cloud backend | `internal/config/settings.go` (defaults) |

## Known limitations

These are open issues, listed here so nobody relies on protection that does not
exist yet.

- **Cloud backends receive plaintext.** With `llm.backend` set to `claude-api`
  or `openai`, or `embedding.backend` set to `voyage`, note content (including
  sensitive notes) is sent to that provider to classify and embed it on
  `inode add`, and decrypted notes are sent to the LLM to answer `inode get`.
  Use the default Ollama backends if this is not acceptable.
- **Summaries of sensitive notes are stored and shown in plaintext.** A summary
  that repeats a secret-looking token from the note (8 or more characters,
  which also catches truncated copies) is replaced with
  `<category> (sensitive): <tags>`, and summaries saved by earlier versions are
  checked once on upgrade (`internal/core/summary.go`). The check looks for
  copied tokens, so a summary can still say what a secret is for, for example
  "Stripe test key".
  [#53](https://github.com/BiSemaphore/inode/issues/53)
- **Embeddings are stored unencrypted** for every note, including sensitive
  ones. Embeddings do not store the text, but they encode its meaning and can
  be partially inverted.
- **API keys in `~/.inode/config.toml` are plaintext.** The file is readable
  only by you (mode 0600), the same model as `~/.aws/credentials` or
  `~/.netrc`, but anything that runs as you, and any backup or dotfiles repo
  that copies the file, can read the keys. To keep keys out of the file, set
  `INODE_LLM_API_KEY` and `INODE_EMBEDDING_API_KEY` in the environment instead.
  [#82](https://github.com/BiSemaphore/inode/issues/82)
- **The SQLite database file** (`~/.inode/notes.db`) is created with the
  default mode (typically 0644); it is protected only by the 0700 `~/.inode`
  directory. Sensitive content inside it is encrypted.
- **Encrypted content is not bound to its note.** The authenticated data is a
  constant, so someone who can write to the database can swap the ciphertext of
  two sensitive notes without detection.
  [#100](https://github.com/BiSemaphore/inode/issues/100)

## Automated checks

- `govulncheck` runs on every pull request and blocks merging when inode's code
  reaches a known vulnerability.
- Dependabot opens weekly update PRs for Go modules and GitHub Actions, and
  security update PRs when an advisory affects a dependency.
- Secret scanning with push protection is enabled on the repository.
