# FlatRun CLI

[English](README.md) | [Français](README.fr.md) | [Español](README.es.md) | [Português do Brasil](README.pt-BR.md) | 简体中文

## 从一个终端管理所有 FlatRun 服务器

`flatrun` 通过与控制面板相同的 API 部署和管理容器应用。它为操作人员提供易读输出，
为脚本提供稳定的 JSON，而且不会引入第二种部署格式。

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/flatrun/cli/main/scripts/install.sh | sudo sh
```

连接服务器并验证连接：

```bash
flatrun profile add production \
  --url https://panel.example.com \
  --token your-api-key-here
flatrun profile use production
flatrun health
```

在终端中，`profile add` 会询问缺少的连接信息，`auth login` 会隐藏密码。脚本仍可通过
参数或标准输入提供这些值。

创建并查看应用：

```bash
flatrun deployment create my-api \
  --image ghcr.io/acme/api:main \
  --port 8080 \
  --host-port 18080
flatrun deployment info my-api
```

删除前先查看计划：

```bash
flatrun deployments delete my-api --plan
```

长时间运行的操作会在终端中显示进度。重定向输出时保持普通文本，`--json` 始终返回
不含交互界面的稳定响应。

从已连接的代理查询操作说明：

```bash
flatrun deployments create --help
flatrun deployments create --generate-cli-skeleton
```

完整命令参考、配置、配置文件和自动化说明请参阅[英文 README](README.md)和
[文档](https://flatrun.dev/docs/cli)。

## 许可证

MIT 许可证。
