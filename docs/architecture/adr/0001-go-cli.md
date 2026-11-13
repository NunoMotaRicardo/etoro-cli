# ADR 0001: Go CLI

Status: accepted (2026-11-13)

## Context
Agents spend many tokens on MCP tool schemas and verbose JSON. A local CLI can return trimmed output and is cheap to describe in a skill. The user requires Go.

## Decision
Implement `etoro-cli` in Go with cobra, go-keyring and golang.org/x/term only. Single static binary, released for linux/darwin/windows on amd64/arm64 with goreleaser; also installable with `go install`. The binary is named `etoro-cli` because `etoro` clashes with an existing npm package.

## Consequences
Easy distribution, fast start, no runtime. Output trimming lives in our code, so new provider fields need curated rendering (mitigated by ADR 0004's generic `api` command).
