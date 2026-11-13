# ADR 0006: Secret storage

Status: accepted (2026-11-13)

## Decision
OS keyring (service `etoro-cli`) with a 0600 JSON file fallback in the config dir; `ETORO_API_KEY`, `ETORO_USER_KEY`, `ETORO_ENVIRONMENT` override both (names compatible with the existing eToro plugin). Keyring calls have a 4 s timeout so a locked or missing secret service cannot hang the CLI. Key values are never printed; `status` shows presence, source, classification and permission.

## Consequences
Headless servers fall back to the file; CI and agents use environment variables. The connect wizard never accepts keys without a TTY.
