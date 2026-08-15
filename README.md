# FlatRun CLI

`flatrun` is the command-line interface for FlatRun. It is intended to be the automation and operator surface for FlatRun in the same way cloud CLIs wrap a larger platform API.

## Install from source

```bash
make build
```

## Quality Checks

```bash
make fmt-check
make vet
make test
make qa
```

CI runs formatting, vet, tests, golangci-lint, and multi-platform builds for Linux and macOS on amd64/arm64.

## Releases

Releases are created from the GitHub Actions **Release** workflow. Before running it:

1. Update `VERSION`.
2. Add the matching entry to `CHANGELOG.md`.
3. Run the workflow manually with the same version value.

The workflow validates that the input version, `VERSION`, and `CHANGELOG.md` agree before calling `whilesmart/workflows/go/release@main`.

## Configuration

Use environment variables in CI:

```bash
export FLATRUN_URL=https://panel.example.com
export FLATRUN_TOKEN=fr_xxx
```

Or configure a local profile:

```bash
flatrun configure set --url https://panel.example.com --token fr_xxx
flatrun configure list
```

Config is stored at `~/.flatrun/config.json` by default. Use `FLATRUN_CONFIG` to override the path.

## Commands

Deploy an existing app from CI by pulling images and then applying a runtime operation:

```bash
flatrun deployment deploy my-app --operation restart --json
```

Create a deployment directly from an image:

```bash
flatrun deployment create my-api \
  --image ghcr.io/acme/api:main \
  --port 8080 \
  --host-port 18080
```

Manage an existing deployment:

```bash
flatrun deployment info my-api
flatrun deployment image set my-api app ghcr.io/acme/api:sha-abc123
flatrun deployment image set my-api app ghcr.io/acme/api:sha-abc123 --deploy --operation restart
flatrun deployment images my-api
flatrun deployment containers my-api
flatrun deployment services my-api
flatrun deployment pull my-api --only-latest
flatrun deployment restart my-api
flatrun deployment rebuild my-api
flatrun deployment stop my-api
flatrun deployment delete my-api
```

`deployment pull` operates at deployment level and may pull multiple images because a deployment can contain multiple compose services and containers. Use `deployment images` to inspect the service-to-image mapping first.

`deployment image set` updates the image for one compose service and writes the updated compose back to the deployment. Add `--deploy` when CI should immediately pull and run a deployment operation after the compose update.

Run commands inside a deployment, for tasks such as database migrations after a release:

```bash
flatrun deployment actions my-api
flatrun deployment action my-api migrate
flatrun deployment exec my-api -- bin/rails db:migrate
flatrun deployment exec my-api worker -- php artisan queue:restart
flatrun container exec abc123 -- sh -c 'printenv | sort'
```

`deployment action` runs a quick action defined on the deployment; `deployment actions` lists them. `deployment exec` runs an ad-hoc command instead: the command follows `--`, and the service is chosen positionally or with `--service` (a single-service deployment is resolved automatically, a multi-service one must be named). Both run in the service container, honor the deployment's protected-mode rules, and surface the command's output (including on a non-zero exit).

### Every other resource

The commands above are shaped by hand because they print tables worth reading. Every other agent
endpoint is `flatrun FAMILY OPERATION [ARGS]`, from a table generated out of the agent's routes.

```bash
flatrun                       # the resources
flatrun backups               # what backups can do
flatrun backup list           # singular and plural both work
flatrun certificates renew shop.example.com
flatrun deployment logs my-api -q service=web -q tail=200
```

Bodies go in as fields or as JSON:

```bash
flatrun domains create -f domain=shop.example.com -f deployment=shop
flatrun settings update --data '{"backups":{"enabled":true}}'
flatrun settings update --data @settings.json
```

A field value that reads as JSON is sent as JSON: `-f enabled=true` sends a boolean, `-f retention=7`
sends a number.

### Driving it from a script or an agent

`--json` on any listing prints every command with its method, path and arguments:

```bash
flatrun --json | jq '.[] | select(.family == "backups")'
flatrun backups --json
```

Add `--json` to any command for the raw response.

### The raw bridge

For anything the table does not cover, such as a streaming endpoint:

```bash
flatrun api get /settings
flatrun api post /databases/list --data '{"container":"mysql"}'
```

Check API connectivity:

```bash
flatrun health
```

See [docs](docs/README.md) for guides and command reference.
