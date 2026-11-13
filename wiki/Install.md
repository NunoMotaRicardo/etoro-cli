# Install

The binary is `etoro-cli` (not `etoro`, which clashes with an npm package).

```bash
go install github.com/NunoMotaRicardo/etoro-cli/cmd/etoro-cli@latest
```

Or download an archive for your OS and architecture (linux, macOS, Windows; amd64, arm64) from the GitHub Releases page, verify it against `checksums.txt`, and put `etoro-cli` on your PATH.

Check: `etoro-cli version`. Then run `etoro-cli connect` in a terminal ([Connect](Connect.md)).

As an agent plugin, install `etoro-cli` from the `nmr-plugins` marketplace; the plugin ships the skill, and the binary must be installed separately.
