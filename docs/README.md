# FlatRun CLI Docs

FlatRun CLI is the automation surface for FlatRun. It is designed for operators, CI systems, and scripts that need to manage deployments without opening the UI.

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
