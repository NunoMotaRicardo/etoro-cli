# Provider compatibility

The eToro Public API state this version of etoro-cli was last verified against. `etoro-cli version` prints the same facts.

| Item | Value |
|---|---|
| Provider | eToro Public API (base `https://public-api.etoro.com/api/v1`) |
| API version (catalog) | v1.415.0 |
| Verified on | 2026-11-13 |
| Route snapshot | `specs/provider/routes-snapshot.json`, 159 routes, hash `fa17f7559991` |
| Source of snapshot | eToro MCP `get-all-routes` (read-only) |
| etoro-cli version | 0.1.0 |

## Known differences from the older REST notes
- Agent portfolios live under `/agent-portfolios` (v2 create/token routes take `scopeNames`); the earlier `/sub-portfolios` notes are outdated. The CLI command is `subportfolios` (alias `agent-portfolios`).
- Newer order routes exist (`/api/v2/trading/execution/orders`, `/api/v3/...` asynchronous). The curated commands use the stable v1 execution routes; use `etoro-cli api` for the newer ones.
- A demo history route exists (`/trading/info/trade/demo/history`); `history` uses it under demo.
- Not verified live (no keys): response field names for search, candles, feeds and people were taken from the REST notes and handled tolerantly (see `contracts/output.md`).

## How to update
After a provider sync (see `docs/plans/provider-sync.md`) update the table above: date, catalog version, route count and hash (`etoro-cli version` prints them).
