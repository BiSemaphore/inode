# Contributing to inode

Thanks for taking the time. This document describes how work actually flows in
this repository: how to set up, how to propose a change, and what a pull request
needs before it can merge. If something here does not match what the repo does,
that is a bug; please open an issue.

---

## Contents

- [Getting started](#getting-started)
- [How work flows](#how-work-flows)
- [Issues](#issues)
- [Branches](#branches)
- [Commits](#commits)
- [Pull requests](#pull-requests)
- [CHANGELOG](#changelog)
- [Versioning and releases](#versioning-and-releases)
- [Labels](#labels)
- [Code standards](#code-standards)
- [Security](#security)

---

## Getting started

### Prerequisites

- **Go 1.26 or newer** (`go.mod` declares `go 1.26.0`; Go supports only its two newest releases, see #98).
- **A C compiler**, because the default storage backend (SQLite + sqlite-vec) uses CGO:
  - macOS: Xcode Command Line Tools (`xcode-select --install`)
  - Linux: `gcc` (for example `sudo apt-get install gcc`)
  - Windows: the MSYS2 MinGW-w64 toolchain with `mingw-w64-x86_64-gcc` and
    `mingw-w64-x86_64-sqlite3`; see the Windows steps in
    [`.github/workflows/ci.yml`](.github/workflows/ci.yml). If you would rather
    avoid the C toolchain, use the Postgres backend below.
- **Ollama** for running inode itself (not needed for the unit tests):
  ```bash
  ollama pull llama3.2
  ollama pull nomic-embed-text
  ```
  No API keys are needed for the default path.

### Build, test, lint

```bash
git clone https://github.com/BiSemaphore/inode.git
cd inode
make build    # CGO_ENABLED=1 go build with version ldflags
make test     # go test -race -cover ./...
make lint     # golangci-lint run ./... (config in .golangci.yml)
```

`make help` lists every target. To try your build: `./inode --help`.

**Do not run the binary with a substitute `HOME`** (for example to test against
a copy of `~/.inode`), especially on macOS: the OS keychain is located through
`HOME`, and the master secret lives there. Tests use `keyring.MockInit()` from
`github.com/zalando/go-keyring` instead of the real keychain; to run the binary
against a copy of a database, set `INODE_DB_PATH` and keep your real `HOME`.

### Postgres backend (optional)

`docker-compose.yml` starts Postgres with pgvector (and Ollama). The Postgres
adapter tests are skipped unless `INODE_TEST_PG_DSN` is set:

```bash
docker compose up -d pgvector
INODE_TEST_PG_DSN="postgres://postgres:password@localhost:5432/postgres?sslmode=disable" \
  go test ./internal/adapters/db/...
```

To run inode against it, set `INODE_DB_BACKEND=postgres` and `INODE_DB_DSN` in
your environment.

---

## How work flows

Every meaningful change follows the same path, so anyone can trace why a line
of code exists and which release shipped it:

```
issue  ->  branch  ->  commits  ->  pull request  ->  CI + review  ->  squash merge
                                        |
                                        +-- CHANGELOG entry (if users can notice the change)

merged changes  ->  release tag  ->  GitHub Release (notes from CHANGELOG)
```

1. **Open or pick an issue** before writing code. Small typo fixes are the only exception.
2. **Branch from `main`** using the naming rule below.
3. **Commit** using Conventional Commits.
4. **Open a pull request** that links the issue with `Closes #N`.
5. **CI must pass**; review comments must be resolved.
6. **Squash merge.** The PR title becomes the single commit on `main`.
7. **Release**: changes ship in the next tagged version, with notes taken from CHANGELOG.

---

## Issues

Work is tracked in GitHub issues, grouped by milestone and shown on the
[inode roadmap board](https://github.com/orgs/BiSemaphore/projects/1).

Every issue has a native **issue type**, and each type has required sections:

| Type | Use it for | Required sections |
|---|---|---|
| **Bug** | Something is broken | Version, environment, steps to reproduce, expected vs actual, severity, root cause (when known), fix approach, acceptance criteria, regression test |
| **Feature** | New user-visible capability | User story, background, in scope, out of scope, acceptance criteria (Given / When / Then), technical notes, blocked by / blocks, definition of done |
| **Task** | Refactors, docs, tooling, process | What and why, in scope, out of scope, done when, blocked by / blocks |
| **Spike** | A design question that must be answered before building | Question, why it blocks, time box, options with trade-offs, deliverable (usually an ADR), what gets filed after |
| **Epic** | A body of work split into child issues | Problem, user stories, goals, non-goals, scope, child issues with dependency order, exit criteria, risks |

Rules:

- **Dependencies are links, not prose.** Use GitHub's "blocked by" relationship
  and sub-issues, so the order of work is visible on the board.
- **If the design is not decided, file a Spike**, not a Feature with guesses in it.
- **Phase is the milestone.** There are no phase labels.
- **Security vulnerabilities are never reported in public issues.** See [Security](#security).

Issue forms that enforce these sections, and a longer writing guide, are being
added in [#68](https://github.com/BiSemaphore/inode/issues/68).

---

## Branches

inode is **trunk-based**: `main` is always releasable and is the only long-lived
branch. There are no integration or release branches.

Name branches `<type>/<issue>-<short-slug>`:

| Example | For |
|---|---|
| `feat/44-teams` | New feature for issue #44 |
| `fix/31-backend-defaults` | Bug fix for issue #31 |
| `docs/66-contributing` | Documentation |
| `refactor/36-principal` | Internal change, no behavior change |
| `test/75-cmd-tests` | Tests only |
| `chore/70-repo-settings` | Tooling, dependencies, housekeeping |
| `ci/69-pr-policy` | CI workflows |

Types are the same as the commit types below. Keep branches short-lived; rebase
on `main` if they fall behind and conflict.

---

## Commits

inode uses [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <summary in imperative mood, lower case, no period>

<body: why the change is needed and what it does, wrapped at ~72 chars>

Closes #N
```

**Types:** `feat`, `fix`, `docs`, `refactor`, `test`, `perf`, `chore`, `ci`, `build`.

**Scopes** (optional, use the area you touched): `add`, `get`, `list`, `note`,
`config`, `mcp`, `search`, `core`, `db`, `llm`, `embedding`, `encryption`, `cli`,
`release`, `deps`, `changelog`, `contributing`.

**Footer:** `Closes #N` when the commit completes an issue, `Part of #N` when it
contributes to one.

**Breaking changes:** append `!` after the type or scope and explain the
migration in a `BREAKING CHANGE:` footer:

```
feat(config)!: rename llm.api_key to llm.key

BREAKING CHANGE: llm.api_key is now llm.key. Run
`inode config set llm.key <value>` after upgrading.
```

Because pull requests are squash-merged, the commits inside a branch can be as
granular as you like; only the PR title and description end up on `main`.

---

## Pull requests

- **One issue per PR.** Put `Closes #N` (or `Part of #N`) at the top of the description.
- **The PR title is a Conventional Commit** (`fix(cli): reject ambiguous note ID prefixes`).
  It becomes the commit message on `main`.
- **Fill in the PR template**: what changed, how you validated it, and the checklist.
- **Run the checks locally** before pushing: `make build`, `make test`, `make lint`.
- **Required checks on `main`**: Test (macos-latest), Test (ubuntu-latest), Lint.
  CI also runs the tests on Windows.
- **Resolve every review conversation** before merging; branch protection
  enforces this.
- **Merge with "Squash and merge"**. Delete the branch afterwards.

A PR that changes behavior without tests, or user-visible behavior without a
CHANGELOG entry and docs, is not ready.

---

## CHANGELOG

[`CHANGELOG.md`](CHANGELOG.md) follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and is written for users, not maintainers.

- **Every change a user can notice adds a line under `## [Unreleased]` in the
  same PR**, in the right section: Added, Changed, Deprecated, Removed, Fixed,
  Security.
- Write what changed from the user's point of view, and link the issue.
- **Repo-only changes** (CI, templates, internal refactors, contributor docs)
  do not need an entry; add the `skip-changelog` label to the PR instead.
- Never edit the section of a version that has already been released.

---

## Versioning and releases

inode uses [Semantic Versioning](https://semver.org/). Until 1.0:

- **Minor** (`0.x.0`): new features, and any breaking change. Breaking changes
  are listed under Changed or Removed in CHANGELOG with migration steps.
- **Patch** (`0.x.y`): bug fixes only.

A released tag is never moved or re-cut: the Go module proxy caches every
version permanently. A mistake in a release is fixed by the next release.

The module path is `github.com/BiSemaphore/inode`. Releases are built by
[`.github/workflows/release.yml`](.github/workflows/release.yml) when a `v*`
tag is pushed. A step-by-step `RELEASING.md` is being added in
[#72](https://github.com/BiSemaphore/inode/issues/72).

---

## Labels

| Label | Meaning |
|---|---|
| `bug` | Something isn't working |
| `enhancement` | New feature or request |
| `documentation` | Improvements or additions to documentation |
| `refactor` | Internal change, no user-visible behavior |
| `chore` | Housekeeping, tooling, release work |
| `security` | Security-relevant change |
| `epic` | Tracking issue for a phase or large feature |
| `process` | How we build: workflow, templates, CI policy, releases |
| `needs-design` | Open design question before implementation |
| `skip-changelog` | Repo-only change, no CHANGELOG entry needed |
| `area/auth`, `area/cli`, `area/core`, `area/db`, `area/embedding`, `area/llm`, `area/mcp`, `area/release`, `area/server` | Part of the codebase affected |
| `priority/high`, `priority/medium`, `priority/low` | Do next, soon, when there is time |
| `good first issue` | Self-contained and well described; good for newcomers |
| `help wanted` | Extra attention is needed |
| `question`, `duplicate`, `invalid`, `wontfix` | Triage outcomes |

Milestones carry the release or phase (`v0.5.0`, `Phase 0: Groundwork`, ...);
there are no phase labels.

---

## Code standards

- `gofmt` and `go vet` clean; `golangci-lint run` clean with the repo's
  [`.golangci.yml`](.golangci.yml).
- **Adapters only.** Databases, LLMs and embedding providers are reached through
  the interfaces in `internal/adapters/{db,embedding,llm}`. Never call an SDK or
  HTTP API directly from `cmd/` or `internal/core/`.
- **Zero cost by default.** The default path (Ollama + SQLite) must keep working
  with no API keys and no paid service. Paid backends are opt-in.
- **Secrets stay secret.** No plaintext secrets written to disk or logs; new
  commands mask sensitive values by default and require confirmation to reveal.
- **Tests come with the change.** Bug fixes include a regression test; features
  include tests for their acceptance criteria. Adapter tests use real SQLite and
  `httptest` servers rather than mocks of our own interfaces where practical.
- **Database changes** need a migration that upgrades existing databases
  without manual steps (versioned migrations are tracked in
  [#85](https://github.com/BiSemaphore/inode/issues/85)).

---

## Security

inode stores secrets. **Do not open a public issue for a vulnerability.**
Follow [SECURITY.md](SECURITY.md) instead.

By participating in this project you agree to follow the
[Code of Conduct](CODE_OF_CONDUCT.md).
