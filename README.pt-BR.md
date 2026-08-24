# CLI do FlatRun

[English](README.md) | [Français](README.fr.md) | [Español](README.es.md) | Português do Brasil | [简体中文](README.zh-CN.md)

## Gerencie todos os servidores FlatRun em um terminal

O `flatrun` implanta e gerencia aplicações em contêineres pela mesma API do
painel. Ele oferece saída legível para operadores e JSON estável para scripts,
sem criar um segundo formato de implantação.

## Instalação

```bash
curl -fsSL https://raw.githubusercontent.com/flatrun/cli/main/scripts/install.sh | sudo sh
```

Conecte um servidor e verifique a conexão:

```bash
flatrun profile add production \
  --url https://panel.example.com \
  --token your-api-key-here
flatrun profile use production
flatrun health
```

Em um terminal, `profile add` solicita os dados ausentes e `auth login` oculta
a senha. Scripts podem continuar fornecendo valores por opções ou pela entrada
padrão.

Crie e inspecione uma aplicação:

```bash
flatrun deployment create my-api \
  --image ghcr.io/acme/api:main \
  --port 8080 \
  --host-port 18080
flatrun deployment info my-api
```

Visualize uma exclusão antes de aplicá-la:

```bash
flatrun deployments delete my-api --plan
```

Operações longas mostram o progresso no terminal. A saída redirecionada
permanece simples e `--json` sempre retorna uma resposta estável sem interface
interativa.

Descubra uma operação no agente conectado:

```bash
flatrun deployments create --help
flatrun deployments create --generate-cli-skeleton
```

Consulte o [README em inglês](README.md) e a
[documentação](https://flatrun.dev/docs/cli) para a referência completa,
configuração, perfis e comandos de automação.

## Licença

Licença MIT.
