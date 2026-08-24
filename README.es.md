# CLI de FlatRun

[English](README.md) | [Français](README.fr.md) | Español | [Português do Brasil](README.pt-BR.md) | [简体中文](README.zh-CN.md)

## Administra todos tus servidores FlatRun desde un terminal

`flatrun` despliega y administra aplicaciones en contenedores mediante la misma
API que el panel. Ofrece una salida legible para operadores y JSON estable para
scripts, sin crear un segundo formato de despliegue.

## Instalación

```bash
curl -fsSL https://raw.githubusercontent.com/flatrun/cli/main/scripts/install.sh | sudo sh
```

Conecta un servidor y verifica la conexión:

```bash
flatrun profile add production \
  --url https://panel.example.com \
  --token your-api-key-here
flatrun profile use production
flatrun health
```

En un terminal, `profile add` solicita los datos que faltan y `auth login`
oculta la contraseña. Los scripts pueden seguir proporcionando valores mediante
opciones o la entrada estándar.

Crea e inspecciona una aplicación:

```bash
flatrun deployment create my-api \
  --image ghcr.io/acme/api:main \
  --port 8080 \
  --host-port 18080
flatrun deployment info my-api
```

Previsualiza una eliminación:

```bash
flatrun deployments delete my-api --plan
```

Las operaciones largas muestran su progreso en un terminal. La salida
redirigida permanece simple y `--json` siempre devuelve una respuesta estable
sin presentación interactiva.

Consulta una operación desde el agente conectado:

```bash
flatrun deployments create --help
flatrun deployments create --generate-cli-skeleton
```

Consulta el [README en inglés](README.md) y la
[documentación](https://flatrun.dev/docs/cli) para la referencia completa, la
configuración, los perfiles y los comandos de automatización.

## Licencia

Licencia MIT.
