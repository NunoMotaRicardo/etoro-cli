# ADR 0005: Environments and safety

Status: accepted (2026-11-13)

## Context
Real trades move real money, trades have no idempotency key, and agents run the CLI.

## Decision
- Two credential profiles, `demo` and `real`. Default environment demo. Real needs `--real` (or `ETORO_ENVIRONMENT=real`) and a stored real key pair.
- Environment guard: `/demo/` paths under a real environment (or trading paths without `demo` under demo) are refused (exit 2). Classification of keys uses `GET /trading/info/real/pnl` (200 real, 403 InsufficientPermissions demo).
- Every state change needs `--yes` (exit 4 with a one-line summary otherwise); real in an interactive terminal also needs a typed "yes". Leverage defaults to 1; SL/TP and optional fields are never sent unless given. Feed posts are public: `--yes` and the text is echoed.
- At-most-once: trade requests are never retried except on 429 (15/30/60 s, max 3). Timeouts, resets, 502/503/504, non-JSON 5xx and unparseable 2xx are ambiguous: wait about 10 s, read the PnL endpoint, report found/not found, exit 5 if still unknown; never resend. Trade calls are spaced at least 3 s (persisted in the cache dir).
- Reads retry 429 (Retry-After) and 5xx/network errors with backoff, max 3. 413/414 halve multi-id batches.

## Consequences
A duplicate position is much worse than a missing one, so the CLI prefers an exit 5 to a retry. Agents must confirm with the user before every `--yes` and `--real`.
