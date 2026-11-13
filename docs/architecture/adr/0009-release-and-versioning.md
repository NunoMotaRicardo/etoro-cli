# ADR 0009: Release and versioning

Status: accepted (2026-11-13)

## Decision
SemVer, tags `vX.Y.Z`, starting at 0.1.0. One version for the binary and all plugin manifests, injected into the binary with `-ldflags` (`internal/version`). `CHANGELOG.md` follows Keep a Changelog. `scripts/release.sh X.Y.Z` bumps manifests, moves Unreleased, commits and tags locally (no push); `scripts/check-versions.sh` runs in CI and on tag builds; `release.yml` runs goreleaser with notes taken from the CHANGELOG section. `etoro-cli version --check` queries the latest GitHub release. Bump rules in `docs/plans/release-process.md`.
