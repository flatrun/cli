# Get Started

## Build

```bash
make build
```

Check the binary:

```bash
./flatrun version
./flatrun help
```

## Configure a Server

Use a profile for local operator work:

```bash
./flatrun configure set --profile prod --url https://panel.example.com --token TOKEN
./flatrun configure use prod
```

Tokens can be read from stdin:

```bash
printf '%s' "$FLATRUN_TOKEN" | ./flatrun configure set --profile prod --url https://panel.example.com --token-stdin
```

Profiles are stored at `~/.flatrun/config.json` by default. Override with `FLATRUN_CONFIG`.

## Verify Access

```bash
./flatrun health
./flatrun deployment list
```

Use `--verbose` to inspect request and response diagnostics:

```bash
./flatrun deployment list --verbose
```
