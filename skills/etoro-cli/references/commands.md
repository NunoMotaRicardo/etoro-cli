# etoro-cli command catalog

Global flags: `--real` `--json` `--raw` `--yes` `--fields a,b` `--limit N`. Default environment demo. Symbols (`AAPL`) or numeric instrument ids are accepted wherever `<SYM|id>` appears; symbols resolve through search and are cached.

## Session
- `connect [--env demo|real]` interactive wizard (user runs it). `status [--check]`, `logout [--env]`, `env`, `version [--check]`.

## Market data
- `search <text> [--exact] [--page N]`
- `quote <SYM|id>...` bid, ask, last, time
- `instrument <SYM|id>`
- `candles <SYM|id> [--interval 1m|5m|10m|15m|30m|1h|4h|1d|1w] [--count N<=1000] [--dir asc|desc]`
- `closing-prices <SYM|id>...`

## Account
- `portfolio`: credit, positions (positionId, symbol, side, units, amount, leverage, openRate, SL, TP), orders (open/limit/close)
- `pnl`: same plus per-position PnL and total unrealized PnL
- `history --since YYYY-MM-DD [--page N --page-size N]`: real key needed for the real endpoint

## Trading (all need `--yes`; real also `--real`)
- `trade open <SYM|id> --buy|--sell (--amount USD | --units N) [--leverage N] [--sl R] [--tp R] [--tsl]`
- `trade close <positionId> [--units N]`
- `order limit <SYM|id> --buy|--sell --rate R (--amount|--units) [--leverage N --sl R --tp R]`
- `order cancel <orderId> --kind open|close|limit`
- Outcomes: `ok: ... id N` (exit 0); landed after unclear answer (exit 0); did not land (exit 1); unknown (exit 5, never resend).

## Social and lists
- `watchlists`, `watchlists show <id>`, `create <name>`, `rename <id> <name>`, `delete <id>`, `items add|remove <id> <SYM|id>... [--type Instrument|Person]`
- `feed instrument <SYM>|--market-id N`, `feed user <username|id>`, `feed post <text>` (public)
- `people <username>...`, `investors search [--period P --sort S --popular -q key=value]`, `investor <username> gain|portfolio|tradeinfo`
- `curated-lists`, `recommendations [n]`
- `subportfolios [list]`, `create --name N --amount USD --token-name T --scope demo:read|... (--token-file F|--show-token)`, `delete <id>`, `token create|delete`

## Any route
- `routes [--search q] [--mutating]` list the snapshot; `routes --diff --against <file|url>` compare.
- `api <GET|POST|PUT|PATCH|DELETE> <path> [-q k=v]... [--body JSON|@file]` with auth, request id, literal commas, environment guard; non-GET needs `--yes`. Paths are relative to `/api/v1` (give `/api/v2/...` in full).

## Exit codes
0 ok, 1 error (incl. 403 permission/KYC), 2 usage (incl. environment mismatch), 3 not connected or 401, 4 confirmation required, 5 ambiguous trade outcome.
