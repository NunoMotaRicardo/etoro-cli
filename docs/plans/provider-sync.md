# Provider sync plan

eToro ships routes often. Run this routine when the eToro changelog or portal shows changes, or at least every month.

1. **Check** the eToro changelog and https://api-portal.etoro.com for new, changed or deprecated routes.
2. **Refresh the snapshot.** With the eToro MCP read-only tools, call `get-all-routes` (no filter) and save the JSON to a file. Never call execute-write, place-trade or place-close. Then:
   ```bash
   etoro-cli routes --diff --against mcp-routes.json      # what changed
   etoro-cli routes --from mcp-routes.json --snapshot > specs/provider/routes-snapshot.json
   ```
   (An OpenAPI JSON file or URL works for `--against` too. eToro has no unauthenticated public route list that we know of; if one appears, use it with `--against <url>`.) The converter stamps `fetchedAt` with today.
3. **Check specs** of changed routes with `get-route-spec` (field names, scopes, rate limits).
4. **Curate**: for routes agents use often, add a curated command (trimmed output) in `internal/cli`, with tests on an `httptest` fake; otherwise leave them to `etoro-cli api`.
5. **Update docs**: `specs/contracts/cli.md`, `wiki/Commands.md`, `skills/etoro-cli/references/commands.md` and `SKILL.md` (keep it under 100 lines).
6. **Record compatibility** in `specs/provider/compatibility.md` (date, catalog version, route count, hash from `etoro-cli version`).
7. **Version and release** per `docs/plans/release-process.md`: new route or curated command is a minor bump, a fix a patch; add the CHANGELOG entry first.

Known drift to watch: v2/v3 order routes, agent-portfolio routes, deprecated routes flagged `[deprecated]` in `etoro-cli routes`.
