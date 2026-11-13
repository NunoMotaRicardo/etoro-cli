# Repo structure — etoro-cli

## Folders

| Key | Folder | Holds | Written by |
|---|---|---|---|
| `specs` | `specs` | Technical specs: normative contracts (`contracts/cli.md`, `auth.md`, `output.md`), `layering-rules.md`, provider snapshot files (`provider/`) | /implement, in the same PR as the code |
| `pm_docs` | `docs` | Planning and design docs: `plans/`, `product/`, `architecture/` (ADRs, code overview), `sources/` | /pm |
| `user_docs` | `wiki` | User documentation: install, connect, commands, safety, troubleshooting, releases | /implement, in the same PR as the user-facing change |

User docs mirrored to the GitHub wiki: yes, by `.github/workflows/wiki-sync.yml`.

## Layers

| Layer | Path | Responsibility |
|---|---|---|
| Wiring | `cmd/etoro-cli/` | `main`: builds the App and runs the CLI |
| CLI | `internal/cli/` | cobra commands, flag handling, confirmation gate, compact rendering, connect wizard |
| Provider client | `internal/etoro/`, `internal/routes/` | HTTP client (auth, request id, retries, trade at-most-once), symbol cache, route snapshot |
| Credentials | `internal/auth/`, `internal/secrets/`, `internal/config/` | which key pair and environment to use, keyring/file store, paths |
| Build info | `internal/version/` | version variables set by goreleaser, update check |

Dependency rule: each layer imports only from layers below it; `cli` is never imported by lower layers. `specs/provider` (embedded snapshot data) is imported by `internal/routes`.

## Where things live

- Application code: `cmd/`, `internal/`
- Tests: next to the code as `*_test.go` (httptest fake eToro server in `internal/etoro` and `internal/cli`).
- Specs, planning docs and user docs: see **Folders** above.
- Skill: `skills/etoro-cli/` (`SKILL.md`, `references/commands.md`). Manifests: `plugin.json`, `.claude-plugin/`, `.cursor-plugin/`, `gemini-extension.json`.
- Release tooling: `scripts/release.sh`, `scripts/check-versions.sh`, `CHANGELOG.md`, `.goreleaser.yaml`.
- Data at rest: credentials in the OS keyring (or `credentials.json`, 0600) and a symbol cache under the user cache dir. Nothing else.

## Entry points

`cmd/etoro-cli/main.go` -> `cli.NewApp` -> `cli.Run` -> cobra command -> `etoro.Client.Do` (guard, auth headers, retries) -> eToro REST API. Output goes through `Doc` rendering (`internal/cli/output.go`).
