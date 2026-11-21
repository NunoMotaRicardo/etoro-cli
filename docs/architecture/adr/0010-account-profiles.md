# ADR 0010: Account profiles

Status: accepted (2026-11-21)

## Context
The CLI held one key pair per environment (demo, real), so it could serve one eToro account. A user managing two accounts needs two sets of keys side by side, and an agent must never send a trade to the wrong one.

## Decision
- A profile is one account with its own demo and real key pairs. `default` is the existing setup, with unchanged storage keys (`demo`, `real`); named profiles store under `<name>/demo` and `<name>/real` in the same keyring service and credentials file.
- Selection by `--profile` or `ETORO_PROFILE` only; no stored "active profile".
- Once more than one profile exists, every credential-using command must name one (exit 2, before any request). Explicit `ETORO_API_KEY`/`ETORO_USER_KEY` runs are exempt because they already name the credentials.
- A small registry file (`profiles.json`) lists profiles because keyrings cannot be enumerated; a pre-profile setup counts as `default` without it.
- Symbol cache and trade-spacing state stay shared (public data; the spacing is a conservative client-side limit).

## Consequences
- No change for single-account users.
- Not supported: fanning one command out over several profiles.
