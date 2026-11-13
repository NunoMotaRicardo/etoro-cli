# Releases

Versions follow SemVer and tags look like `v0.1.0`. One version covers the binary and the plugin manifests. Changes are listed in `CHANGELOG.md`.

- **Install a specific version**: `go install github.com/NunoMotaRicardo/etoro-cli/cmd/etoro-cli@v0.1.0`, or download that release's archive.
- **Upgrade**: `go install github.com/NunoMotaRicardo/etoro-cli/cmd/etoro-cli@latest`, or download the newest archive.
- **Check**: `etoro-cli version` prints version, commit, build date and the eToro API state it was last verified against. `etoro-cli version --check` asks GitHub for the latest release and prints the install command when yours is older.

While the version is 0.x, a breaking flag or exit-code change bumps the minor version and is marked BREAKING in the changelog.
