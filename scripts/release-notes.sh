#!/usr/bin/env bash
# Print the CHANGELOG.md section for one release, for the GitHub release page.
#
#   scripts/release-notes.sh v0.5.0 [CHANGELOG.md]
#
# Prints everything under "## [0.5.0]" up to the next "## [" heading or the
# link references at the bottom, without the heading itself and without the
# trailing "---" separator. Exits non-zero when the section is missing or
# empty, so a release cannot be published without its CHANGELOG entry.
set -euo pipefail

tag="${1:?usage: release-notes.sh vX.Y.Z [CHANGELOG.md]}"
file="${2:-CHANGELOG.md}"
version="${tag#v}"

notes="$(awk -v ver="$version" '
  index($0, "## [" ver "]") == 1 { found = 1; next }
  found && (/^## \[/ || /^\[[^]]+\]: /) { exit }
  found { print }
' "$file" | sed -e '/./,$!d')"  # drop leading blank lines

# Drop trailing blank and "---" lines (portable across GNU and BSD tools).
while [[ -n "$notes" ]]; do
  last="${notes##*$'\n'}"
  [[ "$last" == "$notes" ]] && last="$notes"
  if [[ -z "${last//[[:space:]]/}" || "$last" == "---" ]]; then
    if [[ "$notes" == *$'\n'* ]]; then notes="${notes%$'\n'*}"; else notes=""; fi
  else
    break
  fi
done

if [[ -z "${notes//[[:space:]]/}" ]]; then
  echo "release-notes: no CHANGELOG section for $version in $file" >&2
  exit 1
fi
printf '%s\n' "$notes"
