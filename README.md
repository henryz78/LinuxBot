# LinuxBot

LinuxBot is a minimal BYOK server management assistant for Linux. It runs as a single Go binary, stores state in SQLite, and provides both an interactive CLI and a local-only Web panel.

LinuxBot 是一个极简 BYOK Linux 服务器管理助手。它以 Go 单二进制运行，使用 SQLite 存储状态，同时提供交互式 CLI 和默认仅本机访问的 Web 面板。

## Features / 功能

- Interactive natural-language CLI for server management.
- Embedded Web UI bound to `127.0.0.1` by default.
- BYOK OpenAI-compatible provider configuration.
- Optional Tavily search tool.
- SQLite-backed sessions, runs, steps, messages, approvals, and settings.
- Session switching with persistent mode and working directory.
- Arbitrary shell execution gated by `safe`, `review`, and `open` modes.
- Safe mode uses an explicit allowlist.
- A global critical denylist applies in every mode.
- Web UI collapses execution traces and supports expanded run details.
- Conversation/run deletion removes sensitive run data and invalidates summaries.

- 通过自然语言在交互式 CLI 中管理服务器。
- 内嵌 Web UI，默认绑定 `127.0.0.1`。
- 支持 BYOK OpenAI-compatible 模型配置。
- 可选 Tavily 搜索工具。
- 使用 SQLite 持久化 session、run、step、message、approval 和配置。
- 支持多会话切换，并持久保存模式与工作目录。
- 允许任意 shell，但通过 `safe`、`review`、`open` 三种模式控制。
- Safe mode 使用显式 allowlist。
- 全局 critical denylist 在所有模式下生效。
- Web UI 默认折叠执行过程，可展开查看 run 详情。
- 删除会话 run 时会清理敏感执行数据并使 summary 失效。

## Status / 状态

This repository is currently an MVP implementation. It is intentionally small and local-first. It is not a chat-platform bot, fleet manager, or OpenClaw-style orchestration framework.

当前仓库是 MVP 实现，目标是小、轻、本机优先。它不是聊天平台 bot、集群管理器，也不是 OpenClaw 类编排框架。

## Build / 构建

```bash
go test ./...
go build -o linuxbot ./cmd/linuxbot
```

## Quick Start / 快速开始

List or create sessions:

```bash
./linuxbot sessions
./linuxbot sessions create prod /opt/app
./linuxbot switch prod
```

Configure an OpenAI-compatible provider:

```bash
export OPENAI_API_KEY="your-api-key"
./linuxbot config provider https://api.openai.com <model> "$OPENAI_API_KEY"
```

Configure mode:

```bash
./linuxbot config mode safe
./linuxbot config mode review
./linuxbot config mode open
```

Start the interactive CLI:

```bash
./linuxbot
```

Start the local Web panel:

```bash
./linuxbot web
```

Then open:

```text
http://127.0.0.1:8787
```

In GitHub Codespaces, expose or open port `8787`.

在 GitHub Codespaces 中测试时，打开或转发 `8787` 端口。

## Optional Search / 可选搜索

LinuxBot currently supports Tavily as the only search provider.

```bash
export TAVILY_API_KEY="your-tavily-key"
./linuxbot config search on "$TAVILY_API_KEY"
```

## Modes / 模式

### Safe

Safe mode is the default. Only explicit low-risk commands can run automatically. Commands outside the allowlist require approval. Critical denylist rules still apply.

Safe 是默认模式。只有显式允许的低风险命令可以自动执行。allowlist 之外的命令需要审批。critical denylist 始终生效。

### Review

Review mode requires approval for shell commands. Users can approve once, reject, or mark a command as always approved for that session.

Review 模式下 shell 命令都需要审批。用户可以单次批准、拒绝，或把命令设为当前 session 的 always approve。

### Open

Open mode automatically executes proposed commands, but the global critical denylist still blocks catastrophic commands.

Open 模式会自动执行 AI 提出的命令，但全局 critical denylist 仍会阻止灾难性命令。

## Configuration / 配置

Inspect redacted local configuration:

```bash
./linuxbot config
```

Set session working directory:

```bash
./linuxbot config cwd /opt/app
```

Local data is stored under:

```text
~/.linuxbot/linuxbot.db
```

The database file is created with restrictive permissions where supported.

本地数据保存在：

```text
~/.linuxbot/linuxbot.db
```

数据库文件会尽量以限制权限创建。

## Security Notes / 安全说明

LinuxBot is powerful because it can execute shell commands. It is not a sandbox. Commands run with the permissions of the OS user who started `linuxbot`.

LinuxBot 可以执行 shell，因此能力很强，但它不是沙箱。命令会以启动 `linuxbot` 的系统用户权限执行。

Recommended usage:

- Start in `safe` or `review` mode.
- Use `open` mode only in trusted environments.
- Keep the Web panel bound to `127.0.0.1`.
- Use SSH tunneling or Codespaces port forwarding instead of direct remote exposure.
- Review command traces before deleting runs that may be needed for audit.

建议：

- 默认使用 `safe` 或 `review` 模式。
- 只在可信环境使用 `open` 模式。
- Web 面板保持绑定 `127.0.0.1`。
- 远程访问优先使用 SSH tunnel 或 Codespaces 端口转发。
- 删除 run 前确认是否还需要审计记录。

## Repository Description / 仓库描述

Minimal BYOK Linux server management assistant with interactive CLI, local Web UI, SQLite persistence, policy-gated shell execution, and Tavily search.

极简 BYOK Linux 服务器管理助手，提供交互式 CLI、本机 Web 面板、SQLite 持久化、策略审批 Shell 执行和 Tavily 搜索。
