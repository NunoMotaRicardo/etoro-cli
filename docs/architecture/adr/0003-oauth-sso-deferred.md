# ADR 0003: OAuth SSO deferred

Status: accepted (2026-11-13)

## Context
eToro also supports OAuth SSO (authorization code, `Authorization: Bearer`), but access is granted to partner and enterprise integrations by the eToro partnerships team; individual developers use keys.

## Decision
Out of scope for v1. The auth layer isolates credential resolution (`internal/auth`) and the client sets headers in one place, so a bearer mode can be added later without touching commands. `etoro-cli api` already passes any path, so SSO-only routes work once a token mode exists.

## Consequences
Partner-only routes are not reachable. Revisit if partner access is obtained.
