# Tooling — etoro-cli

## Setup (run in every fresh worktree)

```bash
go mod download
```

## Running things

- Run the CLI: `go run ./cmd/etoro-cli --help`
- Without credentials, `go run ./cmd/etoro-cli portfolio` exits 3 (not connected). For a sandbox set `ETORO_NO_KEYRING=1 ETORO_CLI_CONFIG_DIR=<tmp> ETORO_CLI_CACHE_DIR=<tmp>`.
- Point at a fake server: `ETORO_API_BASE=http://127.0.0.1:PORT`.
- Required tools: Go 1.24+ (CI uses go.mod), bash for `scripts/`, goreleaser only in CI.

## GitHub

- Board / project: none.
- Auth: `gh auth status`.

## Secrets & environment

Variable names only, never values: `ETORO_API_KEY`, `ETORO_USER_KEY`, `ETORO_ENVIRONMENT` (demo|real), `ETORO_NO_KEYRING`, `ETORO_API_BASE`, `ETORO_CLI_CONFIG_DIR`, `ETORO_CLI_CACHE_DIR`. Keys come from the eToro API portal (`etoro-cli connect`).
