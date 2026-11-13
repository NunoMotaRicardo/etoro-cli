# etoro-cli

A Go CLI and Agent Skill that proxy the eToro Public REST API with compact, token-lean output. Demo-first and hard to misuse: state changes need `--yes`, real needs `--real` plus a real key pair, and trades are never resent after an unclear answer.

```bash
go install github.com/NunoMotaRicardo/etoro-cli/cmd/etoro-cli@latest
etoro-cli connect            # guided API key setup (demo + Read recommended)
etoro-cli quote AAPL
etoro-cli portfolio
etoro-cli trade open AAPL --buy --amount 100 --yes     # demo
```

- Curated commands for market data, portfolio, PnL, trading, watchlists, feeds, investors; `etoro-cli api <METHOD> <path>` reaches every route; `etoro-cli routes` lists them.
- Output is compact text by default (`--json`, `--raw`, `--fields`, `--limit` on request).
- Keys live in the OS keyring (or a 0600 file); `ETORO_API_KEY`, `ETORO_USER_KEY`, `ETORO_ENVIRONMENT` override.
- Exit codes: 0 ok, 1 error, 2 usage, 3 not connected, 4 confirmation required, 5 ambiguous trade outcome.
- Published as the `etoro-cli` plugin via the [nmr-plugins](https://github.com/NunoMotaRicardo/nmr-plugins) marketplace (skill in `skills/etoro-cli/`).

Docs: `wiki/` (user), `specs/` (contracts), `docs/` (design, ADRs, plans). Changes: `CHANGELOG.md`. License: MIT. Not affiliated with eToro; trading involves risk.
