#!/usr/bin/env bash
# Verify every manifest carries the same version. On a tag build (GITHUB_REF_TYPE=tag or
# a tag name given as $1) also verify the tag equals that version and CHANGELOG.md has its section.
set -euo pipefail
cd "$(dirname "$0")/.."

files=(plugin.json .claude-plugin/plugin.json .cursor-plugin/plugin.json gemini-extension.json)
version=""
for f in "${files[@]}"; do
  v=$(sed -n 's/^[[:space:]]*"version":[[:space:]]*"\([^"]*\)".*/\1/p' "$f" | head -n1)
  [ -n "$v" ] || { echo "no version in $f" >&2; exit 1; }
  if [ -z "$version" ]; then version=$v; elif [ "$v" != "$version" ]; then
    echo "version mismatch: $f has $v, expected $version" >&2; exit 1
  fi
done
echo "manifests agree: $version"

tag="${1:-}"
if [ -z "$tag" ] && [ "${GITHUB_REF_TYPE:-}" = "tag" ]; then tag="${GITHUB_REF_NAME:-}"; fi
if [ -n "$tag" ]; then
  [ "$tag" = "v$version" ] || { echo "tag $tag does not match manifest version v$version" >&2; exit 1; }
  grep -Eq "^## \[$version\]" CHANGELOG.md || { echo "CHANGELOG.md has no section for $version" >&2; exit 1; }
  echo "tag $tag matches manifests and CHANGELOG"
fi
