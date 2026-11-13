# Connect

`etoro-cli connect` walks you through creating an API key and stores it. Run it yourself in a terminal; never paste keys into a chat.

## What a key is
A key pair has a **Public API Key** and a **User Key**. Each pair is bound to one environment (Virtual/demo or Real) and one permission (Read or Write). Demo and real need separate pairs. Start with demo + Read; choose Write only if you want to trade.

## The wizard
```bash
etoro-cli connect --env demo
```
1. Opens or prints https://api-portal.etoro.com and pauses at each click: log in, Settings > Trading > API Key Management > Create New Key, choose the environment and permission, complete identity verification.
2. Reminds you the **User Key is shown once**: copy both now.
3. Asks for both keys with hidden input (whitespace trimmed, basic shape check).
4. Verifies them: a call that classifies the key as demo or real, and a demo read. If the key is not of the environment you chose, nothing is stored unless you accept the detected one.
5. Detects Read/Write when the API reports it (no trade is ever placed to test it; otherwise it says `unknown` and tells you to check the portal).
6. Stores them in the OS keyring (or a 0600 file when none is available).
7. Shows security advice and next steps.

## Other commands
- `etoro-cli status [--check]`: what is stored, source, classification, permission. Never prints keys.
- `etoro-cli logout [--env demo|real]`: delete stored keys. Also delete the key in the portal.

## CI and agents
`connect` refuses without a terminal. Use environment variables: `ETORO_API_KEY`, `ETORO_USER_KEY`, `ETORO_ENVIRONMENT=demo|real`. They override stored keys.

## Housekeeping
Rotate keys every 90 days, delete old keys in the portal, use a separate key per environment and per tool.
