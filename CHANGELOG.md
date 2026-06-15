# Changelog

All notable changes to the FlatRun CLI are documented in this file.

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
