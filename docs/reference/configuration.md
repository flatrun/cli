# Configuration Reference

## Profiles

Profiles let one workstation or CI runner target multiple FlatRun servers.

```bash
flatrun configure set --profile prod --url https://prod.example.com --token TOKEN
flatrun configure set --profile staging --url https://staging.example.com --token TOKEN
flatrun configure use prod
```

Run a command with a specific profile without switching:

```bash
flatrun deployment list --profile staging
```

## Config File

Default path:

```text
~/.flatrun/config.json
```

Override path:

```bash
FLATRUN_CONFIG=/path/to/config.json flatrun configure list
```

## Environment Variables

| Variable | Purpose |
| --- | --- |
| `FLATRUN_CONFIG` | Override the config file path. |
| `FLATRUN_PROFILE` | Select a profile. |
| `FLATRUN_URL` | Override the FlatRun API URL. |
| `FLATRUN_TOKEN` | Override the FlatRun API token. |

## Security

Prefer `--token-stdin` or `FLATRUN_TOKEN` over `--token` when possible:

```bash
printf '%s' "$TOKEN" | flatrun configure set --profile prod --url https://prod.example.com --token-stdin
```

The config directory is written with `0700` permissions and the config file with `0600`.
