# Troubleshooting

| Symptom | Cause and fix |
|---|---|
| exit 3, "not connected" | No keys for this environment. Run `etoro-cli connect --env demo` (or `real`) in a terminal, or set `ETORO_API_KEY`/`ETORO_USER_KEY`/`ETORO_ENVIRONMENT`. |
| 401 | A key is wrong, expired or swapped. Check both values; regenerate the User Key in the portal. |
| 403 InsufficientPermissions | The key lacks permission: a Read key cannot trade; a demo key cannot use real-only endpoints (`history` on real). Check environment and permission in the portal. Not retried. |
| 403 about verification | Real trading needs identity verification (KYC) on etoro.com. |
| 429 | Rate limited; the CLI already waited and retried. Wait a minute. Trade limit is about 20 per minute. |
| 413/414 | Too many ids; multi-id reads halve automatically, for `api` use fewer ids. |
| "refusing ... endpoint" (exit 2) | Path environment does not match the active one. Use `--real`, or a `/demo/` path. |
| exit 4 | Add `--yes` after confirming with the user. |
| exit 5 | Trade outcome unknown. Do not resend. Check `etoro-cli portfolio`. |
| keyring errors | No secret service: the CLI falls back to a 0600 file. Set `ETORO_NO_KEYRING=1` to skip the keyring. |
| symbol not found | `etoro-cli search <text>` and use the exact symbol or the numeric id. |
| Output looks outdated | `etoro-cli version --check`; `etoro-cli routes --search <word>` shows what the snapshot knows; use `api` for new routes. |
