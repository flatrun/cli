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

Run a quick action:

```bash
flatrun deployment actions my-api
flatrun deployment action my-api migrate
```

`deployment actions` lists the quick actions defined on a deployment. `deployment action` runs one in its service container and prints the command output. Quick actions are configured on the deployment (id, command, and target service) and are subject to the deployment's protected-mode rules. This is how operator commands such as database migrations or cache rebuilds are run from CI.

Run an ad-hoc command (any command the container image provides):

```bash
flatrun deployment exec my-api -- npx prisma migrate deploy
flatrun deployment exec my-api -- bin/rails db:migrate
flatrun deployment exec my-api worker -- python manage.py migrate
flatrun deployment exec my-api --service worker -- php artisan migrate --force
flatrun container exec abc123 -- sh -c 'printenv | sort'
```

`deployment exec` runs a command in a deployment's service container and prints the output. The command and its arguments must follow `--`. Choose the service either as a positional argument (`exec NAME SERVICE -- ...`) or with `--service`; if a deployment has a single running service it is used automatically, and if it has more than one you must name it (otherwise the command reports the available services and stops). `container exec` targets a container directly by ID. Both run non-interactively and are subject to the deployment's protected-mode rules; on a non-zero exit they print the command's captured output and exit non-zero. Use a quick action when you want a named, reusable command instead.

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
