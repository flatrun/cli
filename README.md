# FlatRun CLI

English | [Français](README.fr.md) | [Español](README.es.md) | [Português do Brasil](README.pt-BR.md) | [简体中文](README.zh-CN.md)

## Operate every FlatRun server from one terminal

`flatrun` deploys and manages Docker applications through the same API as the
FlatRun dashboard. It gives operators readable commands and gives automation
stable JSON, without introducing a second deployment format.

Use it to inspect a plan before applying it, manage deployments and
infrastructure, run commands inside containers, push folder contents, follow
jobs, and switch between independent FlatRun servers.

## Start with a server

## Install

Install the latest release on Linux or macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/flatrun/cli/main/scripts/install.sh | sudo sh
```

Connect a server and verify it:

```bash
flatrun profile add production \
  --url https://panel.example.com \
  --token your-api-key-here
flatrun profile use production
flatrun health
```

When run in a terminal, `profile add` prompts for missing connection fields and
stores the token without echoing it. `auth login` prompts for a username and a
masked password. Flags and standard input remain available for scripts and CI.

Create and inspect an application:

```bash
flatrun deployment create my-api \
  --image ghcr.io/acme/api:main \
  --port 8080 \
  --host-port 18080
flatrun deployment info my-api
```

Before a destructive operation, ask the connected agent for its plan:

```bash
flatrun deployments delete my-api --plan
```

The connected agent supplies operation help, accepted request fields, and
request skeletons:

```bash
flatrun deployments create --help
flatrun deployments create --generate-cli-skeleton
```

## Why the CLI exists

The dashboard is useful for exploration. The CLI makes the same work
repeatable from a terminal, a script, or CI. Generated resource commands keep
the complete agent API available while shaped commands make common deployment
work easier to read and remember.

## Install details

The installer detects amd64 or arm64, verifies the published SHA-256 checksum, and installs
`flatrun` in `/usr/local/bin`. Pin a release with `FLATRUN_VERSION`, or choose another destination
with `INSTALL_DIR`.

Update an installed release in place:

```bash
flatrun update
```

Use `sudo flatrun update` when the binary is installed in a system directory such as
`/usr/local/bin`. Check for a newer version without installing it with `flatrun update --check`.

## Build from source

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
flatrun profile add production --url https://panel.example.com --token fr_xxx
flatrun profile use production
flatrun profile list
```

Log in through the agent without placing a password in shell history:

```bash
printf '%s\n' "$FLATRUN_PASSWORD" |
  flatrun auth login --profile production --username operator --password-stdin
```

`configure` remains available as a compatibility alias for profile management.
Config is stored at `~/.flatrun/config.json` by default. Use `FLATRUN_CONFIG` to override the path.

## Terminal output

Human-readable output uses terminal-aware colors and tables. Colors are disabled automatically when output is piped or redirected. Set `NO_COLOR=1` to disable them explicitly. `--json` always returns unstyled machine-readable output.

Long-running writes show a single live progress line in a terminal. Redirected
output and `--json` never start an interactive renderer. Destructive deployment
deletion asks for confirmation in a terminal; automation must continue to use
`--yes` or `--confirm NAME`.

Generate completion for a supported shell:

```bash
flatrun completion bash
flatrun completion zsh
flatrun completion fish
flatrun completion powershell
```

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

### Output

If the agent describes the answer, that decides the layout. Otherwise an array of objects prints as
a table, an array of scalars one per line, an empty one as `None`, and anything else as raw JSON.
Piping changes nothing, so `flatrun deployment list | grep running` works. `--json` is how you ask
for the raw answer.

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

Learn any operation from the connected agent, then print a request body to fill in:

```bash
flatrun deployments create --help
flatrun deployments create --generate-cli-skeleton
```

Operations that declare plan support accept `--plan`. The command returns the changes without
applying them:

```bash
flatrun deployments delete staging --plan \
  -q delete_ssl=true \
  -q delete_database=true \
  -q delete_vhost=true
```

See [docs](docs/README.md) for guides and command reference.
