# Layering rules

```
cmd/etoro-cli
   -> internal/cli
        -> internal/etoro, internal/routes, internal/version
             -> internal/auth -> internal/secrets, internal/config
specs/provider (embedded data)  <- internal/routes
```

1. `cmd/` only wires: it builds the `cli.App` and calls `cli.Run`. No logic.
2. `internal/cli` owns cobra commands, flag parsing, the confirmation gate (`--yes`, typed "yes" for real), exit-code mapping and rendering. It is the only layer that prints.
3. `internal/etoro` is the REST client: URL building (literal commas), headers, request id, environment guard, retries, trade at-most-once and reconciliation, symbol resolution/cache. It returns data and typed errors (`APIError`, `AmbiguousError`, `EnvMismatchError`) and never prints or exits.
4. `internal/routes` parses, searches, matches and diffs route lists. It may import `specs/provider`.
5. `internal/auth` decides the environment and resolves credentials (env vars over store). `internal/secrets` stores key pairs (keyring, 0600 file). `internal/config` resolves paths. These never import `etoro` or `cli`.
6. `internal/version` has no internal imports.
7. Lower layers never import `internal/cli`. Secrets never cross into output: only `cli` formats user-visible text and it only ever sees presence/source/classification.
8. Tests use `httptest`; no package may call the real eToro API in tests.
