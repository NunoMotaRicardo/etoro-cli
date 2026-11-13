# Code overview

Request flow: `cmd/etoro-cli/main.go` builds an `App` (stdio, store, paths, env getter) and calls `cli.Run`. A cobra command resolves the environment (`auth.ActiveEnv`) and credentials (`auth.Resolve`), builds an `etoro.Client`, optionally resolves symbols (`Client.Resolve` with `SymbolCache`), calls the API and renders a `Doc`.

- `internal/etoro/client.go`: `Do` applies the environment guard, trade spacing, headers, retry policy by request `Kind` (read, write, trade). `GetIDs` halves batches on 413/414.
- `internal/etoro/trade.go`: `PlaceTrade` sends once and reconciles ambiguous outcomes with a `Matcher` (open, close, cancel); `Classify` and `Permission` back the connect wizard.
- `internal/etoro/symbols.go`: exact symbol match through `/market-data/search`, 30-day cache.
- `internal/etoro/jx.go`: case-insensitive accessors over generically decoded JSON.
- `internal/cli/output.go`: `Doc` (key/values + tables) rendered as text, JSON or raw; generic `autoDoc` for endpoints without a curated view.
- `internal/cli/connect.go`: the wizard, with injectable input for tests.
- `internal/routes`: snapshot parsing (three formats), search, match, hash, diff.
- `specs/provider`: embedded snapshot data.
