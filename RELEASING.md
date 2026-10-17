# Releasing inode

The steps for cutting a release. Each release is tracked by a "Cut vX.Y.Z"
issue in its milestone; tick these off in that issue.

Versioning is [SemVer](https://semver.org/). Before 1.0: a minor bump
(`0.5.0` to `0.6.0`) for features and breaking changes, a patch bump
(`0.5.0` to `0.5.1`) for fixes only.

## 1. Ready

- [ ] Every issue in the milestone is closed or moved to another milestone,
      with a comment saying why.
- [ ] CI is green on `main`.
- [ ] `CHANGELOG.md` `[Unreleased]` describes every user-visible change since
      the last release (it is written at merge time; this is a read-through).

## 2. Release PR

On a branch `chore/<issue>-release-vX.Y.Z`:

- [ ] `## [Unreleased]` becomes `## [X.Y.Z] - YYYY-MM-DD`; add a new empty
      `## [Unreleased]` above it.
- [ ] Compare links at the bottom: `[Unreleased]` compares `vX.Y.Z...HEAD`,
      and a new `[X.Y.Z]` line compares the previous tag with `vX.Y.Z`.
- [ ] `scripts/release-notes.sh vX.Y.Z` prints the section you expect. This
      is exactly what the release page will show.
- [ ] Anything in README or `docs/` that would be false for this release is
      fixed.
- [ ] PR title `chore(release): vX.Y.Z`, body `Part of #<release issue>`,
      label `skip-changelog`. Merge when CI is green.

## 3. Tag

```bash
git switch main && git pull --ff-only
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

Tags are permanent: the Go module proxy caches every version forever. Never
move or re-push a published tag. If something is wrong after tagging, fix it
on `main` and release the next patch version.

## 4. Publish

The tag push runs `.github/workflows/release.yml`:

1. fails if `go.mod` / `go.sum` are not tidy,
2. extracts the release notes with `scripts/release-notes.sh` (fails if the
   CHANGELOG section is missing or empty),
3. runs goreleaser: linux/amd64 archive, `checksums.txt`, and the release page
   with those notes.

If the workflow fails after the tag exists, fix the cause on `main` and run
the workflow again from the Actions tab (**Release**, **Run workflow**, tag
`vX.Y.Z`). The tag stays as it is.

- [ ] The release page shows the CHANGELOG section, the archive and
      `checksums.txt`.

## 5. Verify

On a machine (or a scratch `GOBIN`) without a local checkout:

```bash
GOBIN=$(mktemp -d) go install github.com/BiSemaphore/inode@vX.Y.Z
"$GOBIN/inode" version     # prints: inode vX.Y.Z (...)
```

- [ ] `go install ...@vX.Y.Z` works and `inode version` prints the tag.
- [ ] `go install ...@latest` resolves to the new version (the proxy can take
      a few minutes).
- [ ] The default path works: with Ollama running, `inode add` and
      `inode get` against a throwaway database
      (`INODE_DB_PATH=$(mktemp -d)/notes.db`). Never point `HOME` at a
      temporary directory on macOS: inode then reads the keychain with a
      different home and the OS shows a keychain reset dialog.

## 6. Close

- [ ] Close the release issue and the milestone.
- [ ] Move the release issue and PR to Done on the board.
