# Release process

## Versioning
SemVer, tags `vX.Y.Z`, first release 0.1.0. One version for the binary and every manifest (`plugin.json`, `.claude-plugin/plugin.json`, `.cursor-plugin/plugin.json`, `gemini-extension.json`). Never hand-edit versions.

## Bump rules
| Change | Bump |
|---|---|
| New provider route support or new curated command | minor |
| Output format change (columns, text layout, JSON shape) | minor |
| Bug fix, docs, internal refactor, provider compatibility refresh with no CLI change | patch |
| Breaking flag or exit-code change | major (while 0.x: minor, marked **BREAKING** in the CHANGELOG) |

## Procedure
1. Make sure `CHANGELOG.md` Unreleased lists every user-visible change (Added, Changed, Fixed, Removed).
2. Quality gates green: `gofmt -l .` empty, `go vet ./...`, `go test ./...`, `go build ./...`.
3. On `main` with a clean tree run `scripts/release.sh X.Y.Z`. It checks main and a clean tree, a valid version greater than the last tag, updates all manifests, moves Unreleased to `## [X.Y.Z] - date`, commits `chore(release): vX.Y.Z` and creates an annotated tag. It does not push.
4. Push: `git push origin main vX.Y.Z`.
5. `release.yml` runs `scripts/check-versions.sh` (tag equals manifests, CHANGELOG has the section), tests, then goreleaser (archives, `checksums.txt`, notes from the CHANGELOG section, ldflags for version/commit/date).
6. Update the marketplace entry only if the plugin source changed shape (versions live in this repo, never in `nmr-plugins`).

## Tie-in with provider sync
A provider sync (`docs/plans/provider-sync.md`) ends in a release: refresh `specs/provider/compatibility.md`, add the CHANGELOG entry (route support is a minor), then run the procedure above.

## Checking for updates
`etoro-cli version --check` compares with the latest GitHub release and prints the install command. CI runs `scripts/check-versions.sh` on every push.
