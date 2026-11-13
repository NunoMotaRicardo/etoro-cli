# Using with agents

The `etoro-cli` skill (in `skills/etoro-cli/`) teaches an agent to use this CLI instead of raw MCP tools, which costs far fewer tokens.

- The agent asks you before every `--yes` and every `--real`, stating environment, instrument, side, amount and leverage.
- If the CLI reports exit 3, you run `etoro-cli connect` yourself in a terminal; keys never enter the chat.
- On exit 5 the agent never resends: it runs `etoro-cli portfolio` and reports.
- It prefers curated commands with the default compact output, uses `--json` only for parsing, and falls back to `etoro-cli api` for anything not curated.
- If behaviour looks outdated it runs `etoro-cli version --check`.

Install the plugin from the `nmr-plugins` marketplace and install the binary ([Install](Install.md)).
