# etoro-cli — eToro Public API CLI and skill

Go CLI (`etoro-cli`) plus an Agent Skill that proxy the eToro Public REST API with token-lean output. Published as a plugin through the `nmr-plugins` marketplace (https://github.com/NunoMotaRicardo/nmr-plugins) to Claude Code, Codex, Copilot CLI, Gemini CLI, Antigravity and Cursor. The binary is `etoro-cli` (not `etoro`, which clashes with an npm package).

`CLAUDE.md` is a symlink to this file. Edit `AGENTS.md` only.

## Principles
- Anything that moves money defaults to the demo environment and needs `--yes`; real needs `--real` and a stored real key pair.
- At-most-once for trades: never resend a trade POST except on 429; reconcile ambiguous outcomes against the portfolio (exit 5 if unknown).
- Never print, log or commit key values. Keys never enter chat.
- Output is the product: compact by default, trimmed fields, `--json`/`--raw` on request.
- Skills are tool-neutral Agent Skills; flag tool-specific features in the skill.

## Layout
```
cmd/etoro-cli/        main
internal/cli          cobra commands, rendering, connect wizard
internal/etoro        REST client, symbol cache, trade reconciliation
internal/routes       committed route snapshot: list, match, diff
internal/{auth,secrets,config,version}
specs/                normative contracts (contracts/), layering-rules.md, provider/ snapshot + compatibility
docs/                 planning: product/, architecture/adr/, plans/, sources/
wiki/                 user docs mirrored to the GitHub wiki
skills/etoro-cli/     the Agent Skill
scripts/              release.sh, check-versions.sh
```
Layering: cmd -> cli -> etoro/routes -> auth/secrets/config (see `specs/layering-rules.md`). Lower layers never import `cli`.

## Conventions
- Before committing: `gofmt -l .` empty, `go vet ./...`, `go test ./...`, `go build ./...`. Tests use `httptest` fakes, never the real API.
- Exit codes are a contract: 0 ok, 1 error, 2 usage, 3 not connected, 4 confirmation required, 5 ambiguous trade outcome.
- Every user-visible change adds a `CHANGELOG.md` entry under Unreleased and updates specs, wiki and the skill together.
- Versions: SemVer, one version across the binary and all manifests (`plugin.json`, `.claude-plugin/`, `.cursor-plugin/`, `gemini-extension.json`). Never hand-edit versions; run `scripts/release.sh X.Y.Z` (it does not push). Bump rules are in `docs/plans/release-process.md`.
- Staying current with eToro: follow `docs/plans/provider-sync.md`; record the verified provider state in `specs/provider/compatibility.md`.
- Never commit API keys, tokens, account ids or personal data.

## Wiki
- `wiki/` is the source of truth; `Home.md` is the landing page. Do not edit the GitHub wiki directly: the `wiki-sync` workflow overwrites it on push to `main`.
- First-time setup: enable Wikis in repo settings and create one page by hand so the wiki repo exists.
