#!/usr/bin/env bash
# Usage: scripts/release.sh X.Y.Z
# Bumps every manifest, moves CHANGELOG "Unreleased" into the new section, commits and tags.
# Does NOT push: it prints the command.
set -euo pipefail
cd "$(dirname "$0")/.."

new="${1:-}"
[[ "$new" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || { echo "usage: $0 X.Y.Z (valid SemVer)" >&2; exit 2; }

[ "$(git rev-parse --abbrev-ref HEAD)" = "main" ] || { echo "must be on main" >&2; exit 1; }
[ -z "$(git status --porcelain)" ] || { echo "working tree is not clean" >&2; exit 1; }

last=$(git tag --list 'v[0-9]*' --sort=-v:refname | head -n1 || true)
last=${last#v}
if [ -z "$last" ]; then
  last=$(sed -n 's/^[[:space:]]*"version":[[:space:]]*"\([^"]*\)".*/\1/p' plugin.json | head -n1)
  # No tag yet: the first release may equal the manifest version.
  greater=$(printf '%s\n%s\n' "$last" "$new" | sort -V | tail -n1)
  [ "$new" = "$last" ] || [ "$greater" = "$new" ] || { echo "$new is lower than $last" >&2; exit 1; }
else
  [ "$new" != "$last" ] && [ "$(printf '%s\n%s\n' "$last" "$new" | sort -V | tail -n1)" = "$new" ] \
    || { echo "$new must be greater than the last tag v$last" >&2; exit 1; }
fi
git rev-parse "v$new" >/dev/null 2>&1 && { echo "tag v$new already exists" >&2; exit 1; }
grep -q '^## \[Unreleased\]' CHANGELOG.md || { echo "CHANGELOG.md has no Unreleased section" >&2; exit 1; }

today=$(date +%Y-%m-%d)
for f in plugin.json .claude-plugin/plugin.json .cursor-plugin/plugin.json gemini-extension.json; do
  sed -i.bak -E "s/^([[:space:]]*\"version\":[[:space:]]*\")[^\"]*(\")/\1$new\2/" "$f" && rm -f "$f.bak"
done

# Move Unreleased into the new section and refresh the compare links
# (skipped when the section already exists, e.g. the very first release 0.1.0).
if ! grep -Eq "^## \[$new\]" CHANGELOG.md; then
tmp=$(mktemp)
awk -v v="$new" -v d="$today" '
  /^## \[Unreleased\]/ && !done { print; print ""; print "## [" v "] - " d; done=1; next }
  { print }
' CHANGELOG.md > "$tmp" && mv "$tmp" CHANGELOG.md
repo="https://github.com/NunoMotaRicardo/etoro-cli"
sed -i.bak -E "s#^\[Unreleased\]:.*#[Unreleased]: $repo/compare/v$new...HEAD#" CHANGELOG.md && rm -f CHANGELOG.md.bak
if [ -n "$(git tag --list "v$last")" ]; then
  echo "[$new]: $repo/compare/v$last...v$new" >> CHANGELOG.md
else
  echo "[$new]: $repo/releases/tag/v$new" >> CHANGELOG.md
fi
fi

scripts/check-versions.sh
git add -A
if ! git diff --cached --quiet; then
  git commit -m "chore(release): v$new" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
fi
git tag -a "v$new" -m "etoro-cli v$new"
echo
echo "Released v$new locally. Review, then push:"
echo "  git push origin main v$new"
