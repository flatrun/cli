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

## Every other resource

The families above are shaped by hand. Every other agent endpoint is
`flatrun FAMILY OPERATION [ARGS]`, from a table generated out of the agent's routes.

```bash
flatrun                     # the families
flatrun backups             # what backups can do
flatrun backups list
flatrun backups restore BACKUP_ID
flatrun certificates renew shop.example.com
```

Operation names follow the endpoint: a collection is `list`, one item is `get`, and a sub-resource
keeps its noun (`log-sources`, `actions`, `jobs`). Where a read and a write share a path, the read
keeps the plain name (`log-sources`, `log-sources-update`). Where a verb applies to one item or to
all of them, the targeted one is plain, so `certificates renew DOMAIN` renews one and
`certificates renew-all` renews everything.

### Sending a body

```bash
flatrun domains create -f domain=shop.example.com -f deployment=shop
flatrun settings update --data '{"backups":{"enabled":true}}'
flatrun settings update --data @settings.json
```

`-f name=value` is repeatable. A value that reads as JSON is sent as JSON, so `-f enabled=true`
sends a boolean and `-f ports=[8080]` sends an array. The two body forms cannot be combined.

### Query parameters

```bash
flatrun deployment logs my-api -q service=web -q tail=200
```

## Listing what exists

```bash
flatrun                       # the families
flatrun backups               # one family
flatrun --json                # every command as JSON
flatrun backups --json        # one family as JSON
```

The JSON gives each command's family, operation, method, path, arguments and exact invocation,
which is what a script or an agent needs to use the CLI without reading this page.

## Raw API

Use the raw API bridge for anything the table does not cover, such as a streaming endpoint:

```bash
flatrun api get /settings
flatrun api get /users
flatrun api post /databases/list --data '{"container":"mysql"}'
```

The path can include or omit `/api`; both `api get /settings` and `api get /api/settings` work.
