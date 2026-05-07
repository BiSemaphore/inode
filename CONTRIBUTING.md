# Contributing to inode

Thank you for your interest in contributing. Please read this document before opening
issues or pull requests.

---

## Table of Contents

- [Getting Started](#getting-started)
- [Branching Strategy](#branching-strategy)
- [Commit Convention](#commit-convention)
- [Pull Request Process](#pull-request-process)
- [Code Standards](#code-standards)
- [Reporting Bugs](#reporting-bugs)
- [Requesting Features](#requesting-features)

---

## Getting Started

```bash
git clone https://github.com/shahid-io/inode.git
cd inode
go mod download
go build ./...
go test ./...
```

**Prerequisites:** Go 1.22+, a Claude API key, a Voyage AI API key.

---

## Branching Strategy

inode follows **Git Flow**. The branching model:

```
main ──────────────────────────────────────────── stable releases (tagged)
  └── develop ──────────────────────────────────── integration branch
        ├── feature/add-command ──────────────────── new features
        ├── feature/voyage-embedding-adapter
        ├── fix/encryption-key-derivation ───────────── bug fixes
        ├── chore/update-deps
        └── docs/update-architecture
              │
              └─▶ release/v1.0.0 ─────────────────── release prep
                    │
                    └─▶ merged into main + develop, tagged v1.0.0
```

### Branch Names

| Prefix | Purpose | Example |
|---|---|---|
| `feature/` | New functionality | `feature/ask-command` |
| `fix/` | Bug fix | `fix/sqlite-vec-query` |
| `hotfix/` | Urgent fix directly on main | `hotfix/key-leak` |
| `release/` | Release preparation | `release/v1.0.0` |
| `chore/` | Dependencies, tooling, config | `chore/update-goreleaser` |
| `docs/` | Documentation only | `docs/add-contributing` |
| `refactor/` | Code restructuring, no behaviour change | `refactor/adapter-interfaces` |
| `test/` | Adding or fixing tests | `test/encryption-unit` |
| `ci/` | CI/CD pipeline changes | `ci/add-windows-build` |

### Rules

- Branch from `develop` for all normal work. Branch from `main` only for `hotfix/`.
- `main` is protected — no direct pushes. Changes arrive only via release merges or hotfixes.
- `develop` is the integration branch — all feature PRs target `develop`.
- Delete branches after merging.

---

## Commit Convention

inode uses **Conventional Commits**. This enables automatic changelog generation
and semantic version bumping.

```
<type>(<scope>): <short description>

[optional body]

[optional footer]
```

### Types

| Type | When to use |
|---|---|
| `feat` | A new feature |
| `fix` | A bug fix |
| `docs` | Documentation only |
| `chore` | Maintenance, dependency updates |
| `refactor` | Refactoring without behaviour change |
| `test` | Adding or fixing tests |
| `perf` | Performance improvement |
| `ci` | CI/CD pipeline changes |
| `build` | Build system or tooling |

### Scopes (optional but encouraged)

`add`, `ask`, `list`, `config`, `note`, `encryption`, `embedding`, `llm`, `db`, `cli`

### Examples

```
feat(ask): implement RAG search pipeline
fix(encryption): use per-note IV instead of global IV
docs(contributing): add branching strategy section
chore(deps): upgrade anthropic-sdk-go to v0.3.0
refactor(db): extract DBAdapter interface to separate file
test(encryption): add AES-256-GCM round-trip test
ci: add Windows build target to release workflow
```

### Breaking Changes

Add `BREAKING CHANGE:` in the commit footer, or append `!` after the type:

```
feat(config)!: rename llm.api_key to llm.key

BREAKING CHANGE: config key renamed from llm.api_key to llm.key
```

Breaking changes trigger a major version bump (v1.0.0 → v2.0.0).

---

## Pull Request Process

1. **Branch** from `develop` (see Branching Strategy above)
2. **Write tests** for your changes
3. **Run checks** locally:
   ```bash
   go test ./...
   go vet ./...
   golangci-lint run
   ```
4. **Open a PR** against `develop` (not `main`)
5. **Fill in the PR template** — describe the change, link related issues
6. **Request review** — at least one approval required before merge
7. **Squash merge** is preferred for feature branches; merge commit for release branches

---

## Code Standards

- Follow standard Go conventions (`gofmt`, `go vet`)
- Run `golangci-lint run` before pushing
- All external dependencies go through adapter interfaces — never call SDKs directly
  from `internal/core/` or `cmd/`
- No plaintext secrets written to disk or logged
- New commands must mask sensitive values by default

---

## Reporting Bugs

Use the [Bug Report](.github/ISSUE_TEMPLATE/bug_report.md) template.
For security vulnerabilities, see [SECURITY.md](SECURITY.md) — **do not open a public issue**.

## Requesting Features

Use the [Feature Request](.github/ISSUE_TEMPLATE/feature_request.md) template.
Check the [roadmap in README.md](README.md#roadmap) first to see if it's already planned.
