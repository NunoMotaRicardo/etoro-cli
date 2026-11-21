# Profiles: more than one eToro account

Use a profile per eToro account you manage from the same machine. Each profile has its own **demo and real** key pairs, so the environment (`--real`) works inside a profile exactly as before.

## Set up

```
etoro-cli connect                          # your existing keys stay the "default" profile
etoro-cli --profile client-a connect       # guided key setup for the second account
etoro-cli --profile client-a connect --env real   # its real keys, when you need them
etoro-cli profiles                         # default: demo (keyring), client-a: demo (keyring)
```

Names: 1-32 lowercase letters, digits, `-` or `_`. Create each key pair in the portal while logged in as that account.

## Use

```
etoro-cli --profile client-a portfolio
etoro-cli --profile client-a --real trade open AAPL --buy --amount 100 --yes
export ETORO_PROFILE=client-a              # for a whole shell session
```

Once more than one profile exists, every command that uses credentials needs `--profile` or `ETORO_PROFILE`; otherwise it stops with exit code 2, before any request, and lists the profiles. This is deliberate: a trade can never silently go to the wrong account. Trade confirmations name the account: `[client-a/real] ...`.

Not needed: `version`, `help`, `completion`, `profiles`, `routes`, and runs where `ETORO_API_KEY` and `ETORO_USER_KEY` are set (explicit credentials; they override stored keys for any profile).

## Remove

`etoro-cli --profile client-a logout` deletes that profile's stored keys. Delete the keys in the eToro portal too.

## Where things live

| What | Default profile | Named profile |
|---|---|---|
| Keyring account (service `etoro-cli`) | `demo`, `real` | `<name>/demo`, `<name>/real` |
| Fallback file | `<config>/etoro-cli/credentials.json`, same keys | |
| Profile list | `<config>/etoro-cli/profiles.json` | |

The symbol cache and trade spacing are shared across profiles.
