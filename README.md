# etoro-cli

A Go CLI and Agent Skill that proxy the eToro Public REST API with compact, token-lean output. Demo-first and hard to misuse: state changes need `--yes`, real needs `--real` plus a stored real key pair, and trades are never resent after an unclear answer.

- **Curated commands** for market data, portfolio, PnL, trading, watchlists, feeds, investors, and sub-portfolios.
- **Generic escape hatch**: `etoro-cli routes` lists the API route snapshot; `etoro-cli api <METHOD> <path>` reaches any route.
- **Token-lean output**: compact text by default; `--json`, `--raw`, `--fields`, `--limit` on request.
- **Safety gates**: demo environment by default; `--yes` required on every state change (exit 4); at-most-once trades with portfolio reconciliation (exit 5 if unknown).
- **Secure key storage**: OS keyring with a `0600` file fallback (`ETORO_API_KEY`, `ETORO_USER_KEY`, `ETORO_ENVIRONMENT` override).

## Install the CLI

### With Go

Requires Go 1.26 or newer:

```bash
go install github.com/NunoMotaRicardo/etoro-cli/cmd/etoro-cli@latest
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`. To install a specific release, replace `@latest` with `@v0.1.0`.

### From GitHub Releases

Download the archive for your OS and architecture (`linux`, `darwin`, `windows`; `amd64`, `arm64`) from [Releases](https://github.com/NunoMotaRicardo/etoro-cli/releases), verify it against `checksums.txt`, unpack it, and put `etoro-cli` on your `PATH`.

### Verify

```bash
etoro-cli version
etoro-cli --help
```

## Quick start (CLI)

```bash
etoro-cli connect                          # guided API key setup in a terminal (demo + Read recommended)
etoro-cli status                           # check stored credentials and classification
etoro-cli search "apple"                   # find instrument id / symbol
etoro-cli quote AAPL BTC                   # live bid/ask/last
etoro-cli candles AAPL --interval 1d --count 30
etoro-cli portfolio                        # demo positions and pending orders
etoro-cli pnl                              # positions with live PnL
etoro-cli trade open AAPL --buy --amount 100 --yes   # demo trade
etoro-cli routes --search watchlist        # discover any eToro route
etoro-cli api GET /market-data/exchanges   # call any route directly
```

Exit codes: `0` ok, `1` error, `2` usage, `3` not connected or 401, `4` confirmation required (`--yes`), `5` ambiguous trade outcome (never resend; check `etoro-cli portfolio`).

## Install and use the Agent Skill

The Agent Skill lives in [`skills/etoro-cli/`](skills/etoro-cli/) and is published as the `etoro-cli` plugin through the [nmr-plugins](https://github.com/NunoMotaRicardo/nmr-plugins) marketplace.

### 1. Install the skill

**Claude Code**

```bash
claude plugin marketplace add NunoMotaRicardo/nmr-plugins
claude plugin install etoro-cli@nmr-plugins
```

**Codex**

```bash
codex plugin marketplace add NunoMotaRicardo/nmr-plugins
codex plugin add etoro-cli@nmr-plugins
```

**GitHub Copilot CLI**

```bash
copilot plugin marketplace add NunoMotaRicardo/nmr-plugins
copilot plugin install etoro-cli@nmr-plugins
```

**Cursor** — add `NunoMotaRicardo/nmr-plugins` as a plugin marketplace and install `etoro-cli`.

**Gemini CLI**

```bash
gemini extensions install https://github.com/NunoMotaRicardo/etoro-cli
```

**Antigravity / other Agent Skills tools** — install from `https://github.com/NunoMotaRicardo/etoro-cli`, or copy `skills/etoro-cli/` into your agent's skills directory.

### 2. Connect and use with an agent

1. Install the `etoro-cli` binary (above) so it is on the agent's `PATH`.
2. Run `etoro-cli connect` yourself in a terminal (never paste API keys into a chat; if an agent hits exit 3, it will ask you to run `etoro-cli connect`).
3. Ask your agent normally — for example: *"What is AAPL trading at on eToro?"*, *"Show my eToro demo portfolio and PnL"*, or *"Open a $100 demo buy on AAPL"*. The skill instructs the agent to use compact commands and ask for explicit confirmation before every `--yes` and `--real`.

## Docs

- User guide: [wiki](wiki/Home.md) ([GitHub Wiki](https://github.com/NunoMotaRicardo/etoro-cli/wiki))
- Technical contracts: [specs/](specs/)
- Design, ADRs and plans: [docs/](docs/Home.md)
- Changes: [CHANGELOG.md](CHANGELOG.md)

License: [MIT](LICENSE). Not affiliated with eToro; trading involves risk.
