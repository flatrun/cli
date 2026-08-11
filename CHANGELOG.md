# Changelog

All notable changes to the FlatRun CLI are documented in this file.

## [0.3.0] - 2026-08-11

### Added

- Every endpoint the agent exposes is now a command, as `flatrun FAMILY OPERATION [ARGS]`, covering 294 endpoints across 42 resource families including backups, certificates, databases, domains, security, scheduler, object stores, users and API keys. The command table is generated from the agent's own routes rather than written by hand, so the CLI reaches a new endpoint as soon as it is regenerated instead of trailing behind by a release.
- `flatrun commands` lists every command, and `flatrun commands --json` prints the same list with each command's method, path and arguments, so a script or an agent can discover the whole surface without reading the docs.
- `flatrun` with no arguments and `flatrun FAMILY` with no operation list what is available at that level.
- Request bodies can be built with repeatable `-f name=value` fields, or passed whole with `--data JSON` or `--data @file.json`. A field value that reads as JSON is sent as JSON, so `-f enabled=true` sends a boolean rather than a string. Query parameters go in with repeatable `-q name=value`.

### Fixed

- Single-dash flags with values (`-url`, `-token`, `-data`) were parsed as though they took no value, so the following argument was swallowed. Both spellings now work, as the flag package intends.

## [0.2.0] - 2026-06-15

### Added

- `flatrun deployment actions NAME` lists the quick actions defined on a deployment.
- `flatrun deployment action NAME ACTION_ID` runs a quick action in its service container and prints the command output, enabling operator commands such as database migrations and cache rebuilds to run from CI.
- `flatrun deployment exec NAME [SERVICE] -- COMMAND [ARGS...]` runs an ad-hoc command in a deployment's service container and prints the output, for one-off operator commands that are not defined as quick actions. The service may be named positionally or with `--service`; a single-service deployment is resolved automatically, while a deployment with more than one service must be told which to use rather than guessing.
- `flatrun container exec CONTAINER_ID -- COMMAND [ARGS...]` runs an ad-hoc command in a container by ID.
- Exec and quick action failures print the command's captured output alongside the error, so a non-zero exit shows what the container actually reported instead of only the exit status. The command still exits non-zero.

## [0.1.0] - 2026-05-23

### Added

- Initial standalone `flatrun` CLI for FlatRun automation and operator workflows.
- Profile-based configuration stored in `~/.flatrun/config.json`.
- Deployment commands for list, info/get, create, delete, runtime operations, deploy, pull, images, containers, and services.
- Image and container command families.
- Raw API bridge via `flatrun api get|post|put|delete`.
- Human-readable table output by default, with `--json` for raw machine-readable responses.
- `--verbose` diagnostics for request method, URL, response status, and response body size.
- Manual GitHub release workflow for multi-platform CLI binaries.

### Changed

- `deployment info` is the preferred human-facing detail command; `deployment get` remains available as an alias.
- Deployment list and info output include multi-domain, SSL, certificate, proxy, healthcheck, database, and service details where the API returns them.
