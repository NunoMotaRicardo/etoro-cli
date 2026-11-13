# ADR 0007: Staying current with the provider

Status: accepted (2026-11-13)

## Context
eToro has no public unauthenticated route list that we know of. The eToro MCP server exposes `get-all-routes` and `get-route-spec` read-only.

## Decision
Commit `specs/provider/routes-snapshot.json` (generated, embedded in the binary). `etoro-cli routes --diff --against <file|url>` compares it with another list in snapshot, MCP `get-all-routes` or OpenAPI format; `routes --from <file> --snapshot` converts a list into snapshot JSON. The routine (check changelog/portal, refresh the snapshot via MCP, add curated commands, update skill, record compatibility, bump version, release) is in `docs/plans/provider-sync.md`. `specs/provider/compatibility.md` records the verified state and `etoro-cli version` shows it.

## Consequences
Provider drift is detected by a diff and handled in a small, repeatable release.
