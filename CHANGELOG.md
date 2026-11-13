# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/). While the version is 0.x, a breaking
change to a flag or exit code bumps the minor version and is marked **BREAKING**.

## [Unreleased]

## [0.1.0] - 2026-11-13

### Added
- `etoro-cli` Go CLI for the eToro Public REST API with compact, token-lean output (`--json`, `--raw`, `--fields`, `--limit`).
- Guided `connect` wizard (demo and real key pairs, hidden input, live verification and demo/real classification), `status`, `logout`, `env`.
- Curated commands: `search`, `quote`, `instrument`, `candles`, `closing-prices`, `portfolio`, `pnl`, `history`, `trade open|close`, `order limit|cancel`, `watchlists`, `feed`, `people`, `investors`, `investor`, `curated-lists`, `recommendations`, `subportfolios`.
- Generic `api <METHOD> <path>` for any route and `routes` (committed snapshot, `--diff --against`).
- Safety: demo by default, `--real` plus stored real keys for real, `--yes` on every state change (exit 4), at-most-once trade handling with portfolio reconciliation (exit 5), client-side trade spacing and 429 handling.
- Secrets in the OS keyring with a 0600 file fallback; `ETORO_API_KEY`, `ETORO_USER_KEY`, `ETORO_ENVIRONMENT` overrides.
- Agent Skill `skills/etoro-cli`, plugin manifests, `version --check`, release tooling.

[Unreleased]: https://github.com/NunoMotaRicardo/etoro-cli/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/NunoMotaRicardo/etoro-cli/releases/tag/v0.1.0
