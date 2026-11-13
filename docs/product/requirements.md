# Requirements

## Goals
- Let agents (and people) use eToro from a terminal with far fewer tokens than raw MCP tool schemas and raw JSON.
- Demo-first and hard to misuse: no accidental real trade, no duplicated trade.
- Cheap to keep current as eToro ships routes.

## Users
Agent users of Claude Code, Codex, Copilot CLI, Gemini CLI, Antigravity, Cursor via the Agent Skill; developers and traders at a terminal.

## Functional requirements
1. Curated commands for market data, portfolio/PnL/history, trading (open, close, limit, cancel), watchlists, feeds, people/investors, lists, recommendations, sub-portfolios, environment info.
2. Generic `api` for any route, `routes` for the snapshot, `routes --diff` for drift.
3. Guided `connect` wizard, `status`, `logout`; two environments with separate key pairs.
4. Safety: demo default, `--real`, `--yes` (exit 4), typed confirmation on real in a terminal, environment guard, at-most-once trades with reconciliation (exit 5).
5. Output modes: compact text, `--json`, `--raw`, `--fields`, `--limit`.
6. Symbol resolution with an on-disk cache.
7. Release tooling and `version --check`.

## Non-functional requirements
- Single static binary on linux/darwin/windows (amd64/arm64).
- Never print or log key values; keys at rest in the keyring or a 0600 file.
- Respect rate limits (trade spacing, 429 handling).
- Tests with `httptest`, no network; CI on three operating systems.

## Token-saving rationale
A raw MCP session pays for tool schemas up front and returns full JSON (portfolio responses repeat dozens of fields per position). `etoro-cli` returns one line per position with about ten columns, resolves symbols so the agent never juggles ids, and a skill under 100 lines replaces schemas. `--json` is for parsing only.

## Out of scope (v1)
OAuth SSO (ADR 0003), partner-only routes, streaming prices, a TUI.
