# Writing issues for inode

An issue is the unit of work in this repository. A good one lets someone who
was not in the conversation build the thing, test it, and know when it is
done, without asking. This guide explains what each issue type must contain
and why. The [issue forms](../.github/ISSUE_TEMPLATE/) enforce the required
fields; this page explains how to fill them well.

---

## Choosing a type

| Type | Use it when | Example |
|---|---|---|
| [Bug](#bug) | Something behaves differently from the docs, the help text, or common sense | [#90](https://github.com/BiSemaphore/inode/issues/90) short ID prefixes match an arbitrary note |
| [Feature](#feature) | Users get a new capability they can notice | [#91](https://github.com/BiSemaphore/inode/issues/91) `inode note edit` |
| [Spike](#spike) | A design question must be answered before anyone can write a Feature | [#83](https://github.com/BiSemaphore/inode/issues/83) note permission model |
| [Task](#task) | Refactors, docs, tooling, CI, process: work with no new user-facing capability | [#66](https://github.com/BiSemaphore/inode/issues/66) rewrite CONTRIBUTING |
| [Epic](#epic) | A body of work split into several child issues | [#76](https://github.com/BiSemaphore/inode/issues/76) engineering process |

**Feature or Spike?** If you cannot write the acceptance criteria because the
approach is undecided ("should this be per-note ACLs or mode bits?"), it is a
Spike. File the Feature after the spike decides. Writing a Feature full of
guesses produces work that gets thrown away.

---

## Rules for every issue

1. **Evidence over adjectives.** Cite files and lines (`internal/core/tagger.go:37`),
   commands and their output, issue numbers. "Search is slow" is not actionable;
   "`inode get` takes 4.1 s on 2k notes with Ollama on an M1" is.
2. **Scope both ways.** Say what is in and what is deliberately out, and where
   the out-of-scope work is tracked.
3. **Dependencies are links.** After creating the issue, use the sidebar:
   - **Relationships → Blocked by** for issues that must close first
   - **Sub-issues** on the parent epic
   Prose like "depends on the auth work" is not enough; the board cannot see it.
4. **Milestone is the phase or release** (`v0.5.0`, `Phase 0: Groundwork`, `Backlog`).
   There are no phase labels.
5. **Labels**: one of `bug` / `enhancement` / `documentation` / `refactor` / `chore`,
   plus `area/*`, `priority/*`, and `security` where relevant. Full list in
   [CONTRIBUTING](../CONTRIBUTING.md).
6. **No secrets.** Replace real keys, tokens and DSNs with fake values in every
   field, including logs.
7. **Security vulnerabilities are never public issues.** Follow
   [SECURITY.md](../SECURITY.md).

### Sizing

| Size | Meaning |
|---|---|
| S | One sitting |
| M | A few sittings |
| L | Too big: split it, or file a spike first |

---

## Bug

Goal: someone else can reproduce it on the first try and knows what "fixed" means.

| Section | What to write |
|---|---|
| Version | `inode version` output, or the commit if built from source |
| Environment | OS and architecture; `db.backend`, `llm.backend`, `embedding.backend` |
| Steps to reproduce | Exact commands from a state that can be recreated (an empty `HOME` is ideal) |
| Expected | What should happen, and where that is promised (README, help text, CHANGELOG, SECURITY.md) |
| Actual | Full output, including errors |
| Severity | high: data loss, secret exposure, or a documented path that does not work; medium: wrong behavior with a workaround; low: cosmetic |
| Root cause | File and line if found; otherwise leave for the fixer |
| Fix approach | How to fix it, once known |
| Acceptance criteria | Given / When / Then statements the fix must satisfy |
| Regression test | The test that would have caught it |

Example: [#53](https://github.com/BiSemaphore/inode/issues/53) shows a bug with
two reproduction paths, a table of every affected code path, and a data
migration in the fix approach.

---

## Feature

Goal: the reader knows who it is for, what exactly to build, and how to prove it works.

| Section | What to write |
|---|---|
| User story | "As a <kind of user>, I want <capability>, so that <outcome>." The "so that" is the point; if it is weak, the feature probably is too |
| Background | How things work today, with file paths, and why that is not enough |
| In scope | Commands, flags, config keys, behavior this issue delivers |
| Out of scope | Related work deliberately excluded, and where it lives |
| Acceptance criteria | Given / When / Then, one per line, each testable |
| Technical notes | Approach, files to touch, existing functions to reuse, risks |
| Blocked by / Blocks | Mirror of the native links, for readers of the text |
| Related | Other relevant issues and PRs |
| Definition of done | Tests, docs (README, `docs/`), CHANGELOG line under Added, CI green |
| Size | S / M / L |

Acceptance criteria are the most important part. Good:

> Given two notes sharing the prefix `3`, when running `inode note delete 3`,
> then nothing is deleted and both candidates are listed.

Not testable:

> Deleting notes should be safer.

---

## Spike

Goal: a decision, recorded where future readers will find it, plus the list of
work it unlocks.

| Section | What to write |
|---|---|
| Question | One sentence |
| Why it blocks | The issues or decisions waiting on the answer |
| Time box | Effort allowed before deciding with what is known ("one working session") |
| Context | Facts that constrain the answer: code, measurements, prior decisions |
| Options | Each option with trade-offs (a table works well), and which way you lean |
| Work | Prototypes, measurements, reading |
| Deliverable | Usually an ADR in `docs/adr/`; sometimes a decision comment on the issue |
| Exit criteria | Checklist: measured, decided, recorded |
| What gets filed after | The Features and Tasks the decision unlocks |

A spike that runs out of time still ends with a decision, even a provisional
one, written down with what remains unknown.

---

## Task

Goal: the reader understands why the work matters and can check that it is done.

| Section | What to write |
|---|---|
| What and why | The problem with evidence; a "says vs reality" table works well for docs drift |
| In scope | What will change |
| Out of scope | What will not, and where it is tracked |
| Done when | A checklist someone else can verify |
| Blocked by / Blocks | Mirror of the native links |
| Related | Other relevant issues |

---

## Epic

Goal: anyone can see why the work exists, what finishing means, and the order
in which the children must land.

Required sections: Problem, User stories, Goals (measurable), Non-goals (with
reasons), Scope (in and out, naming the covering issue), Child issues
(a table: wave, issue, size, blocked by, blocks), Exit criteria, Definition of
done for children, Risks (risk, impact, mitigation), Decisions or Open
questions (each with a recommendation), Related.

After creating an epic:
1. Attach every child as a **sub-issue**.
2. Add **blocked by** links between children so the waves in the table are real.
3. Keep the child table and the sub-issues in sync when scope changes.

Reference: [#76](https://github.com/BiSemaphore/inode/issues/76) (process) and
[#60](https://github.com/BiSemaphore/inode/issues/60) (Phase 0 groundwork).

---

## When the issue is done

Pull requests link their issue with `Closes #N` (or `Part of #N`), and merging
closes it. If an issue turns out to be wrong, close it with a comment saying
why and what replaces it, rather than leaving it open and stale.
