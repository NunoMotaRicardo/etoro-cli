# Using with agents

The `etoro-cli` skill (in `skills/etoro-cli/`) teaches an agent to use this CLI for eToro instead of large REST schemas or uncurated API calls, keeping token usage low and enforcing safety gates.

## Setup

1. Install `etoro-cli` ([Install](Install.md)) so it is on the agent's `PATH`.
2. Run `etoro-cli connect` yourself in a terminal (demo + Read is recommended to start; keys never enter the chat).
3. Install the skill via the [nmr-plugins](https://github.com/NunoMotaRicardo/nmr-plugins) marketplace (`claude plugin install etoro-cli@nmr-plugins`, `codex plugin add etoro-cli@nmr-plugins`, `copilot plugin install etoro-cli@nmr-plugins`), `gemini extensions install https://github.com/NunoMotaRicardo/etoro-cli`, or by copying `skills/etoro-cli/` into your agent's skills folder.

## How to use it

Ask your agent directly in natural language, for example:
- *"Check the current eToro quote for AAPL and BTC."*
- *"Show my eToro demo portfolio and unrealized PnL."*
- *"Open a $100 demo buy on AAPL."*

## What the agent does

- **Demo-first and confirmed**: the agent defaults to demo and asks you before every `--yes` and every `--real`, stating environment, instrument, side, amount or units, and leverage.
- **No keys in chat**: if the CLI reports exit 3, the agent asks you to run `etoro-cli connect` yourself in a terminal.
- **At-most-once trades**: on exit 5 (ambiguous trade outcome) the agent never resends — it runs `etoro-cli portfolio` and reports.
- **Token-lean**: it prefers curated commands with the default compact output, uses `--json` only for parsing, and falls back to `etoro-cli routes` and `etoro-cli api` for anything not curated.
- **Up-to-date**: if behaviour looks outdated it runs `etoro-cli version --check`.
