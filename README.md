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

Call any backend endpoint while a polished command is still pending:

```bash
flatrun api get /settings
flatrun api post /databases/list --data '{"container":"mysql"}'
flatrun api post /deployments/my-api/actions/migrate
```

Check API connectivity:

```bash
flatrun health
```

See [docs/COMMAND_MAP.md](docs/COMMAND_MAP.md) for the CLI coverage map.
