# Install

The binary is `etoro-cli` (not `etoro`, which clashes with an npm package).

## Install the CLI

### With Go

Requires Go 1.26 or newer:

```bash
go install github.com/NunoMotaRicardo/etoro-cli/cmd/etoro-cli@latest
```

The binary is placed in `$(go env GOPATH)/bin`, which must be on your `PATH`. To pin a specific version, replace `@latest` with a tag such as `@v0.1.0`.

### From GitHub Releases

Download an archive for your OS and architecture (`linux`, `darwin`, `windows`; `amd64`, `arm64`) from the [GitHub Releases](https://github.com/NunoMotaRicardo/etoro-cli/releases) page, verify it against `checksums.txt`, unpack it, and put `etoro-cli` on your `PATH`. Archives also include `skills/etoro-cli/`.

### Verify

```bash
etoro-cli version
etoro-cli --help
```

Then run `etoro-cli connect` in a terminal ([Connect](Connect.md)).

## Install the Agent Skill

The skill in `skills/etoro-cli/` teaches AI agents to use `etoro-cli` instead of guessing flags or schemas. Installing the plugin ships the skill; install the `etoro-cli` binary separately (above) so the agent can run it on `PATH`.

- **Claude Code**:
  ```bash
  claude plugin marketplace add NunoMotaRicardo/nmr-plugins
  claude plugin install etoro-cli@nmr-plugins
  ```
- **Codex**:
  ```bash
  codex plugin marketplace add NunoMotaRicardo/nmr-plugins
  codex plugin add etoro-cli@nmr-plugins
  ```
- **GitHub Copilot CLI**:
  ```bash
  copilot plugin marketplace add NunoMotaRicardo/nmr-plugins
  copilot plugin install etoro-cli@nmr-plugins
  ```
- **Cursor**: add `NunoMotaRicardo/nmr-plugins` as a plugin marketplace and install `etoro-cli`.
- **Gemini CLI**:
  ```bash
  gemini extensions install https://github.com/NunoMotaRicardo/etoro-cli
  ```
- **Antigravity / other Agent Skills tools**: install from `https://github.com/NunoMotaRicardo/etoro-cli`, or copy `skills/etoro-cli/` into your agent's skills directory.

Next: [Connect](Connect.md) and [Using with agents](Using-with-agents.md).
