# ADR 0004: Hybrid command surface

Status: accepted (2026-11-13)

## Context
eToro ships routes often (the catalog had 159 at v1.415.0, including v2 and v3 routes). Curated commands give the best token savings but lag; a generic path never lags.

## Decision
Two tiers. (a) Generic and always current: `etoro-cli api <METHOD> <path> [-q k=v] [--body]` with comma joining, request id, auth, the environment guard and the same write gate; `etoro-cli routes` lists the committed snapshot. (b) Curated commands with trimmed output for the common jobs: search, quote, instrument, candles, closing-prices, portfolio, pnl, history, trade, order, watchlists, feed, people, investors, curated-lists, recommendations, subportfolios, env. Anything not curated stays reachable through (a); promote frequently used routes to (b) during provider sync.

## Consequences
New provider features are usable immediately via `api`. The curated set grows by minor releases.
