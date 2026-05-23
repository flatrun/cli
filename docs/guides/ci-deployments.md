# Deploy from CI

Use CI secrets as environment variables instead of writing a local profile:

```yaml
env:
  FLATRUN_URL: ${{ secrets.FLATRUN_URL }}
  FLATRUN_TOKEN: ${{ secrets.FLATRUN_TOKEN }}
```

## Update One Service Image

For compose-based deployments, update a single service image:

```bash
flatrun deployment image set my-api app ghcr.io/acme/api:sha-abc123
```

To update compose, pull, and run the deployment operation in one command:

```bash
flatrun deployment image set my-api app ghcr.io/acme/api:sha-abc123 --deploy --operation restart
```

Use `--operation rebuild` when containers must be recreated from the updated compose:

```bash
flatrun deployment image set my-api app ghcr.io/acme/api:sha-abc123 --deploy --operation rebuild
```

## Pull Existing Images

If the compose already references a moving tag such as `latest`, pull and restart:

```bash
flatrun deployment deploy my-api --operation restart
```

`deployment deploy` pulls first by default. Use `deployment images` before deployment to inspect every service-to-image mapping:

```bash
flatrun deployment images my-api
```

## Machine-Readable Output

Use `--json` for scripts:

```bash
flatrun deployment image set my-api app ghcr.io/acme/api:sha-abc123 --json
```

Use `--verbose` for diagnostics. It prints request URL, status, and response body size without printing the bearer token.
