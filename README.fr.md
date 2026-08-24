# CLI FlatRun

[English](README.md) | Français | [Español](README.es.md) | [Português do Brasil](README.pt-BR.md) | [简体中文](README.zh-CN.md)

## Gérez tous vos serveurs FlatRun depuis un terminal

`flatrun` déploie et gère des applications conteneurisées avec la même API que
le tableau de bord. Il fournit une sortie lisible aux opérateurs et un JSON
stable aux scripts, sans créer un second format de déploiement.

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/flatrun/cli/main/scripts/install.sh | sudo sh
```

Connectez un serveur et vérifiez la connexion :

```bash
flatrun profile add production \
  --url https://panel.example.com \
  --token your-api-key-here
flatrun profile use production
flatrun health
```

Dans un terminal, `profile add` demande les informations manquantes et
`auth login` masque le mot de passe. Les scripts peuvent continuer à fournir
les valeurs avec des options ou l'entrée standard.

Créez et inspectez une application :

```bash
flatrun deployment create my-api \
  --image ghcr.io/acme/api:main \
  --port 8080 \
  --host-port 18080
flatrun deployment info my-api
```

Prévisualisez une suppression :

```bash
flatrun deployments delete my-api --plan
```

Les opérations longues affichent leur progression dans un terminal. Une sortie
redirigée reste simple et `--json` renvoie toujours une réponse stable sans
présentation interactive.

Découvrez une opération depuis l'agent connecté :

```bash
flatrun deployments create --help
flatrun deployments create --generate-cli-skeleton
```

Consultez le [README anglais](README.md) et la
[documentation](https://flatrun.dev/docs/cli) pour la référence complète, la
configuration, les profils et les commandes d'automatisation.

## Licence

Licence MIT.
