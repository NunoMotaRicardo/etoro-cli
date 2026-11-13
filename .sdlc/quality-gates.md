# Quality gates — etoro-cli

## Gates

- Commands: see `quality_gates.*` in `sdlc-config.yml` (test, lint, format_check, type_check).
- CI: `.github/workflows/ci.yml` runs the same commands on ubuntu, macos and windows, plus `scripts/check-versions.sh`. Local gates and CI must not disagree.
- Security scanning (SAST/dependency audit): not configured; secrets handling is covered by tests (no key values in output) and review. Add `govulncheck` here if introduced.

## Merge requirements

- CI green on all three operating systems.
- User-visible changes carry a `CHANGELOG.md` entry under Unreleased, and docs/specs/wiki updated in the same PR.
- Tests never touch the real eToro API; everything uses `httptest` fakes.
