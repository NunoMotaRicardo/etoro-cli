# CLI contract

Binary `etoro-cli`. Global flags (all commands): `--real`, `--json`, `--raw`, `--yes`, `--fields a,b`, `--limit N`.

## Environment and credentials
- Active environment: `--real` or `ETORO_ENVIRONMENT=real` selects real; otherwise **demo**.
- Credentials: `ETORO_API_KEY` + `ETORO_USER_KEY` (labelled by `ETORO_ENVIRONMENT`, default: the active environment) override the stored pair; otherwise the stored pair for the active environment. None: exit 3.
- Environment guard: a path with a `demo` segment under a real environment, or a `/trading/` path without one under demo, is refused before sending (exit 2).

## Exit codes
| Code | Meaning |
|---|---|
| 0 | ok |
| 1 | provider or runtime error (incl. 403 permission/KYC, trade confirmed not landed) |
| 2 | usage error (bad flag, refused environment mismatch, `connect` without a TTY) |
| 3 | not connected, or 401 (keys wrong/expired): run `etoro-cli connect` |
| 4 | confirmation required (no `--yes`, or typed answer was not "yes") |
| 5 | ambiguous trade outcome, still unknown after reconciliation: never resend |

## State-changing commands
`trade open|close`, `order limit|cancel`, `watchlists create|rename|delete|items add|remove`, `feed post`, `subportfolios create|delete|token create|delete`, and `api` with any method other than GET (except read-only POST routes in the snapshot). Without `--yes`: nothing is sent, one line on stderr `error: confirmation required: [env] <summary>; re-run with --yes to execute`, exit 4. With `--yes` on real in an interactive terminal the summary is shown and the user must type `yes`.

## Commands
| Command | Behaviour |
|---|---|
| `connect [--env demo\|real]` | guided wizard (see `auth.md`); needs a TTY |
| `status [--check]` | per environment: connected/not, source (env, keyring, file), classification, permission; `--check` classifies live. Never prints keys |
| `logout [--env]` | delete stored keys (both without `--env`) |
| `env` | active environment and key state |
| `version [--check]` | version, commit, date, provider compatibility line; `--check` queries GitHub latest release |
| `api <METHOD> <path> [-q k=v]... [--body JSON\|@file\|-]` | any route; path relative to `/api/v1` or a full `/api/vN/...`; commas literal; compact JSON out |
| `routes [--search q] [--mutating] [--from file] [--snapshot] [--diff --against file\|url]` | committed route snapshot; diff vs MCP/OpenAPI/snapshot JSON |
| `search <text> [--exact] [--page N]` | instruments: id, symbol, name, market |
| `quote <SYMBOL\|id>...` | bid/ask/last/time; symbols resolved via search and cached |
| `instrument <SYMBOL\|id>` | scalar display fields |
| `candles <SYMBOL\|id> [--interval 1m..1w\|API name] [--count N<=1000] [--dir asc\|desc]` | date open high low close volume |
| `closing-prices <SYMBOL\|id>...` | historical closes |
| `portfolio` / `pnl` | credit, unrealized PnL, positions, pending orders (open/limit/close); `pnl` adds per-position PnL |
| `history --since YYYY-MM-DD [--page N --page-size N]` | trade history; the real endpoint needs a real key (clear message on `InsufficientPermissions`) |
| `trade open <SYM\|id> --buy\|--sell (--amount N\|--units N) [--leverage N] [--sl R] [--tp R] [--tsl]` | market open; leverage default 1; SL/TP only if given |
| `trade close <positionId> [--units N]` | close by position id; full unless `--units` |
| `order limit <SYM\|id> --buy\|--sell --rate R (--amount\|--units) ...` | market-if-touched order |
| `order cancel <orderId> --kind open\|close\|limit` | cancel pending order |
| `watchlists`, `watchlists show <id>`, `create <name>`, `rename <id> <name>`, `delete <id>`, `items add\|remove <id> <SYM\|id>... [--type Instrument\|Person]` | watchlists |
| `feed instrument <SYM> \| --market-id N`, `feed user <username\|id>`, `feed post <text>` | feeds (post is public; text echoed) |
| `people <username>...` | profiles/CIDs |
| `investors search [--period P --sort S --popular -q k=v]`, `investor <user> gain\|portfolio\|tradeinfo` | popular investors |
| `curated-lists`, `recommendations [n]` | lists |
| `subportfolios [list]`, `create ...`, `delete <id>`, `token create\|delete ...` | agent portfolios (`/agent-portfolios`; v2 create). Token secret only via `--token-file` or `--show-token` |

## Trade handling (at-most-once)
- Trade POST/DELETE under `/trading/execution/` is sent once. Retried only on 429 (waits 15, 30, 60 s, max 3). Calls are spaced at least 3 s apart (persisted across processes).
- Timeout, reset, 502/503/504, non-JSON 5xx or unparseable 2xx: wait about 10 s, read the PnL endpoint and look for the trade (new position/order for instrument+side; position gone for close; order gone for cancel). Found: exit 0 and say so. Not found: exit 1 "did not land, not resent". Read failed: exit 5.
- 4xx (except 429) and 5xx with a JSON body: error shown, exit 1, never resent.

## Read handling
429 honours `Retry-After` (else exponential backoff with jitter), 5xx and network errors back off; max 3 retries. 413/414 on multi-id reads halve the batch.
