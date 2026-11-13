# ADR 0002: Direct REST with API keys

Status: accepted (2026-11-13)

## Context
eToro offers a REST Public API (API keys) and an MCP server (browser sign-in). We want deterministic, scriptable, low-token access.

## Decision
Call the REST API directly with `x-api-key`, `x-user-key` and a fresh `x-request-id` (UUID) per request. Never combine with a Bearer token (eToro rejects it). Array query params are joined with literal commas (`%2C` is rejected).

## Consequences
No MCP dependency. Keys are bound to one environment and permission, which shapes the two-profile model (ADR 0005). Request id is regenerated on retries.
