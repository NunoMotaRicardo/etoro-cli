# Development

```bash
go mod download
gofmt -l .        # must print nothing
go vet ./... && go test ./... && go build ./...
go run ./cmd/etoro-cli --help
```
Tests use `httptest` fakes; never call the real eToro API. Sandbox env vars: `ETORO_NO_KEYRING=1`, `ETORO_CLI_CONFIG_DIR`, `ETORO_CLI_CACHE_DIR`, `ETORO_API_BASE` (point at a fake server).

Layers: cmd -> internal/cli -> internal/etoro, internal/routes -> internal/auth, secrets, config (`specs/layering-rules.md`).

## Updating to new provider features
eToro ships routes often. The routine is in `docs/plans/provider-sync.md`: diff the route list (`etoro-cli routes --diff --against <file>`), refresh `specs/provider/routes-snapshot.json`, add curated commands for what agents use, update specs, wiki, skill and `specs/provider/compatibility.md`, then release. Anything new is usable immediately through `etoro-cli api`.

## Releasing
SemVer, one version across binary and manifests. Add a CHANGELOG entry under Unreleased for every user-visible change, then `scripts/release.sh X.Y.Z` (no push) and `git push origin main vX.Y.Z`. CI runs `scripts/check-versions.sh`; tag builds run goreleaser. See `docs/plans/release-process.md`.
