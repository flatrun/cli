# FlatRun CLI Docs

FlatRun CLI is the automation surface for FlatRun. It is designed for operators, CI systems, and scripts that need to manage deployments without opening the UI.

## Get Started

- [Install and configure](guides/get-started.md)
- [Deploy from CI](guides/ci-deployments.md)
- [Release the CLI](guides/releases.md)

## Reference

- [Command reference](reference/commands.md)
- [Configuration](reference/configuration.md)

## Design Notes

The CLI is resource-first: commands start with the thing being managed, then the action.

```bash
flatrun deployment list
flatrun deployment info my-api
flatrun deployment image set my-api app ghcr.io/acme/api:sha-abc123
flatrun image list
flatrun container list
```

Human-readable tables are the default. Use `--json` when a command is feeding another tool.
