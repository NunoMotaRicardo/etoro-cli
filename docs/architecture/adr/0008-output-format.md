# ADR 0008: Compact output and tolerant parsing

Status: accepted (2026-11-13)

## Decision
Default output is compact plain text (aligned columns, `key: value`), trimmed to useful fields; `--json` gives compact JSON of the trimmed data, `--raw` the provider response, `--fields` and `--limit` narrow it. Responses are decoded generically and read case-insensitively (`instrumentId`/`instrumentID`, `positionID`...), with array discovery by shape, so an envelope or casing change does not break rendering.

## Consequences
Less code per endpoint and fewer tokens; trade-off is that field names for endpoints we could not verify live are guessed tolerantly (listed in `specs/provider/compatibility.md`).
