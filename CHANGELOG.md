# Changelog

All notable changes to the FlatRun CLI are documented in this file.

## [0.3.0] - 2026-08-11

### Added

- Every agent endpoint is now a command: `flatrun FAMILY OPERATION [ARGS]`, covering 294 endpoints across 42 families. The table is generated from the agent's routes, so catching up is a regeneration rather than 294 hand-written wrappers.
- `flatrun` lists the resources, `flatrun RESOURCE` lists its commands, with singular and plural both accepted, and `--json` on either prints the same list with each command's method, path and arguments, for scripts and agents. One listing covers both the hand-shaped commands and the generated ones, and the singular families reach everything their plural counterparts do, so `deployment log-sources` works.
- Request bodies from repeatable `-f name=value`, or `--data JSON` / `--data @file.json`. A field value that reads as JSON is sent as JSON, so `-f enabled=true` sends a boolean. Query parameters with repeatable `-q name=value`.

- Commands read the agent's own description of its API where the agent serves one, so a mistyped field or query parameter fails before the request with the name it was probably meant to be, `COMMAND --help` lists the fields an endpoint takes and the permission it needs, and answers print as tables laid out from the types the agent returns. An agent that does not describe itself behaves as before.

- An answer nothing has described is still laid out: the field holding the rows becomes a table, an array of plain values prints one per line, and an empty one reads `None`. `--json` prints the raw answer, as before.

### Fixed

- `-url`, `-token` and other single-dash flags swallowed the following argument, because only the double-dash spelling was registered as taking a value.
- Path arguments were not escaped, so a value containing a slash reshaped the request path.

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
