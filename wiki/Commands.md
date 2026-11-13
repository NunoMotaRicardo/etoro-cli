# Commands

Every command accepts `--json` (compact JSON), `--raw` (provider response), `--fields a,b`, `--limit N`, `--real`, `--yes`. Run `etoro-cli <command> --help` for flags. Default environment is demo.

## Market data
`search <text> [--exact]` · `quote AAPL BTC ...` · `instrument AAPL` · `candles AAPL --interval 1d --count 30 --dir desc` · `closing-prices AAPL`

## Account
`portfolio` · `pnl` · `history --since 2026-01-01` (real key) · `env` · `status`

## Trading (needs `--yes`)
```bash
etoro-cli trade open AAPL --buy --amount 100            # demo, leverage 1
etoro-cli trade open AAPL --sell --units 2 --leverage 2 --sl 150 --tp 120
etoro-cli trade close 123456789 [--units 1]
etoro-cli order limit AAPL --buy --rate 150 --amount 100
etoro-cli order cancel 987 --kind limit
```
Without `--yes` nothing is sent (exit 4). Stop-loss, take-profit and leverage above 1 are only sent when you give them.

## Social and lists
`watchlists` (`show`, `create`, `rename`, `delete`, `items add|remove`) · `feed instrument AAPL` / `feed user name` / `feed post "text"` (public) · `people name...` · `investors search --period LastYear` · `investor name gain|portfolio|tradeinfo` · `curated-lists` · `recommendations 10` · `subportfolios` (`create`, `delete`, `token create|delete`)

## Any route
```bash
etoro-cli routes --search watchlist          # find a route
etoro-cli api GET /market-data/instruments/rates -q instrumentIds=1001,1002
etoro-cli api POST /watchlists -q name=Ideas --yes
```

## Exit codes
0 ok, 1 error, 2 usage, 3 not connected, 4 confirmation required, 5 ambiguous trade outcome.
