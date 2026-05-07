# Security Policy

inode stores secrets, API keys, and sensitive developer data. Security issues are treated
with the highest priority.

## Supported Versions

| Version | Supported |
|---|---|
| latest (`main`) | ✅ |
| older releases | security fixes backported on a case-by-case basis |

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Report vulnerabilities via email: **security@[your-domain]** (or direct GitHub private
security advisory).

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

You will receive a response within **48 hours**. If the issue is confirmed, a fix will be
prioritised and a CVE requested where applicable.

## Security Design

| Concern | Mitigation |
|---|---|
| Secrets at rest | AES-256-GCM encryption; key derived via Argon2id from OS keychain secret |
| Key storage | OS keychain (macOS Keychain / Linux Secret Service / Windows Credential Locker) |
| Config file | `~/.inode/config.toml` stored at mode 600; API keys encrypted |
| Sensitive output | Values masked by default; `--reveal` requires explicit confirmation |
| Embeddings | Semantic vectors only — raw content cannot be reconstructed from them |
| Transport (Phase 2) | HTTPS only |
| Authentication (Phase 2) | Argon2id password hashing; JWT with short expiry + refresh tokens |
