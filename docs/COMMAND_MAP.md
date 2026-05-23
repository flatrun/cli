# FlatRun CLI Command Map

The CLI should eventually cover every action exposed by the FlatRun UI. Commands are grouped by resource first, then operation.

## Current Coverage

| UI/API area | CLI commands |
| --- | --- |
| Deployment lifecycle | `deployment create`, `deployment delete`, `deployment list`, `deployment info`, `deployment get` |
| Deployment runtime | `deployment start`, `deployment stop`, `deployment restart`, `deployment rebuild`, `deployment deploy` |
| Deployment multi-image/service/container visibility | `deployment images`, `deployment services`, `deployment containers` |
| Deployment image pull | `deployment pull` pulls all images in the deployment compose stack. Use `deployment images` first to see service-to-image mapping. |
| Global images | `image list`, `image pull`, `image delete` |
| Containers | `container list`, `container start`, `container stop`, `container restart`, `container delete` |
| Any backend endpoint | `api get`, `api post`, `api put`, `api delete` |

## Escape Hatch

Until a resource has a polished command, use the raw API bridge:

```bash
flatrun api get /settings
flatrun api get /users
flatrun api post /databases/list --data '{"container":"mysql"}'
flatrun api post /deployments/my-app/actions/migrate
```

The path can include or omit `/api`; both `api get /settings` and `api get /api/settings` work.

## Planned Resource Families

These should become first-class commands as the CLI matures:

| Resource | Proposed command family |
| --- | --- |
| Deployments | `deployment <operation>` |
| Domains | `domain <list|add|update|delete>` |
| Files | `file <list|get|put|delete|mkdir|mount>` |
| Environment | `env <list|set|delete|sync>` |
| Backups | `backup <list|create|restore|delete|download>` |
| Databases | `db <test|list|create|delete|query|user|grant>` |
| Users and API keys | `user <operation>`, `apikey <operation>` |
| Registries and credentials | `registry <operation>`, `credential <operation>` |
| Certificates and proxy | `cert <operation>`, `proxy <operation>` |
| Networks and volumes | `network <operation>`, `volume <operation>` |
| Infrastructure | `infra <operation>` |
| Security, traffic, audit | `security <operation>`, `traffic <operation>`, `audit <operation>` |
| Scheduler | `schedule <operation>` |
| DNS and cluster | `dns <operation>`, `cluster <operation>` |
