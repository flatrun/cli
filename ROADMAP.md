# Roadmap

The CLI should eventually cover every action exposed by the FlatRun UI.

## Planned Resource Families

| Resource | Proposed command family |
| --- | --- |
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
