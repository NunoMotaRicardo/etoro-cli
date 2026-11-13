# Auth contract

## Mechanism
Direct REST with API keys: headers `x-api-key` (Public API Key), `x-user-key` (User Key), `x-request-id` (fresh UUID v4 per request, also per retry). Bearer tokens are never sent alongside key headers. OAuth SSO is out of scope for v1 (ADR 0003).

## Profiles
Two profiles, `demo` and `real`, each one key pair. A key pair is bound by eToro to one environment and one permission (Read or Write).

## Storage
- OS keyring, service `etoro-cli`, account = profile, value = JSON `{api_key, user_key, permission, classified, saved_at}`.
- Fallback when the keyring is unavailable or `ETORO_NO_KEYRING` is set: `os.UserConfigDir()/etoro-cli/credentials.json`, directory 0700, file 0600, same JSON keyed by profile.
- Overrides (highest priority): `ETORO_API_KEY`, `ETORO_USER_KEY`, `ETORO_ENVIRONMENT`. The label decides which environment the variables serve (default: the active one); variables labelled for the other environment are ignored.
- Key values are never printed, logged or echoed; `status` shows presence, source, classification, permission.

## Classification
`GET /trading/info/real/pnl`: 200 = real key; 403 `InsufficientPermissions` = demo key; other 403 = unknown (likely KYC); 401 = invalid keys. Permission is read from `GET /api/v1/me` scopes when present (`:write` anywhere = write, only `:read` = read), else `unknown`. No trade is ever placed to test permissions.

## connect flow (TTY only)
1. Explain keys, environment and permission binding; recommend demo + Read.
2. Walk the portal: https://api-portal.etoro.com, Settings > Trading > API Key Management > Create New Key > environment > permission > identity verification, with an Enter pause per step; offer to open the browser.
3. Warn the User Key is shown once.
4. Prompt for both keys with hidden input; trim whitespace; shape check (>= 16 chars, no whitespace/quotes, the two differ); 3 attempts.
5. Verify: classify, then a demo read (`/trading/info/demo/pnl`) for demo keys. Mismatch with the chosen environment: explain, offer to store under the detected one, otherwise store nothing. 401/403/KYC/429 map to a fix.
6. Detect permission (see above), store under the profile, report keyring or file.
7. Security advice (never paste in chat, rotate every 90 days, delete old keys in the portal, separate keys per environment/service) and next steps.
Without a TTY: exit 2 explaining the environment variables.

## Refresh
API keys do not refresh. Rotation: create a new key in the portal, run `etoro-cli connect` again (replaces the stored pair), delete the old key in the portal.
