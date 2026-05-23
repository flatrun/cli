# Command Reference

## Global Options

| Option | Purpose |
| --- | --- |
| `--profile NAME` | Use a named profile. |
| `--url URL` | Override the FlatRun API URL. |
| `--token TOKEN` | Override the FlatRun API token. |
| `--timeout DURATION` | Request timeout. |
| `--insecure-skip-verify` | Skip TLS certificate verification. |
| `--json` | Print raw JSON instead of friendly output. |
| `--verbose` | Print request/response diagnostics to stderr. |

## Configure

```bash
flatrun configure set --profile prod --url https://panel.example.com --token TOKEN
flatrun configure set --profile prod --url https://panel.example.com --token-stdin
flatrun configure list
flatrun configure use prod
flatrun configure delete prod
```

## Health

```bash
flatrun health
```

## Deployments

List deployments:

```bash
flatrun deployment list
```

Show deployment details:

```bash
flatrun deployment info my-api
flatrun deployment get my-api
```

`deployment get` is an alias for `deployment info`.

Create a deployment:

```bash
flatrun deployment create my-api --image ghcr.io/acme/api:main --port 8080 --host-port 18080
```

Update one compose service image:

```bash
flatrun deployment image set my-api app ghcr.io/acme/api:sha-abc123
flatrun deployment image set my-api app ghcr.io/acme/api:sha-abc123 --deploy --operation restart
```

Inspect deployment resources:

```bash
flatrun deployment images my-api
flatrun deployment services my-api
flatrun deployment containers my-api
```

Pull deployment images:

```bash
flatrun deployment pull my-api
flatrun deployment pull my-api --only-latest
```

Run deployment operations:

```bash
flatrun deployment start my-api
flatrun deployment stop my-api
flatrun deployment restart my-api
flatrun deployment rebuild my-api
flatrun deployment deploy my-api --operation restart
```

Delete a deployment:

```bash
flatrun deployment delete my-api --confirm my-api
flatrun deployment delete my-api --yes
```

## Images

```bash
flatrun image list
flatrun image pull ghcr.io/acme/api:sha-abc123
flatrun image pull ghcr.io/acme/api:sha-abc123 --credential-id cred_123
flatrun image delete IMAGE_ID
```

## Containers

```bash
flatrun container list
flatrun container start CONTAINER_ID
flatrun container stop CONTAINER_ID
flatrun container restart CONTAINER_ID
flatrun container delete CONTAINER_ID
```

## Raw API

Use the raw API bridge while a polished command is still pending:

```bash
flatrun api get /settings
flatrun api get /users
flatrun api post /databases/list --data '{"container":"mysql"}'
```

The path can include or omit `/api`; both `api get /settings` and `api get /api/settings` work.
