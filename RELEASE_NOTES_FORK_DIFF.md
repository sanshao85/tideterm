# TideTerm Fork Diff Release Notes

> 用途：这份文档用于对外发布（GitHub Release/README 链接）和开源合规说明，概括 TideTerm 相对 Wave Terminal 上游的主要新增能力与行为差异。

## 1) 比较基线

- 上游项目：`wavetermdev/waveterm`
- 上游基线提交：`90011a7ede0931046f4c0843e9af027dcea8eefe`（2025-12-22）
- 当前 fork 版本参考：`v0.13.1-tideterm.1`（HEAD `d442b036`）
- 说明日期：2026-02-12

## 2) 本 Fork 的主要新增功能

### 2.1 单个终端块内多会话（Multi-session Terminals）

- 在一个终端块内可创建多个终端会话。
- 提供会话列表侧栏（可显示/隐藏、切换会话、结束单个会话）。
- 侧栏宽度可拖拽调整并持久化。
- 新建会话时，优先继承当前激活会话的连接与工作目录。

代码落点：

- `frontend/app/view/term/term-model.ts`
- `frontend/app/view/term/term.tsx`
- `pkg/waveobj/metaconsts.go`
- `pkg/waveobj/wtypemeta.go`

### 2.2 远程 tmux 会话管理器

- 新增远程 tmux 会话管理弹窗。
- 支持会话列表、最近活跃时间、当前块标识。
- 支持 **Attach / Force Attach / Rename / Kill / Kill All**。

代码落点：

- `frontend/app/modals/tmuxsessions.tsx`
- `pkg/wshrpc/wshserver/tmux.go`

### 2.3 内置 API 代理（WaveProxy）

- 新增 **API Proxy** 视图块与 Dock 状态组件。
- 支持 `messages` / `responses` / `gemini` 三类通道配置。
- 每个通道可配置多 API Key、Base URL、优先级、启用状态。
- 支持通道健康检查（Ping）、指标统计、请求历史。
- 包含调度策略（亲和/故障熔断/自动切换）与本地配置持久化。
- 支持 `wsh proxy` 命令启动代理。

代码落点：

- `frontend/app/view/proxy/proxy.tsx`
- `frontend/app/view/proxy/proxy-dock.tsx`
- `pkg/waveproxy/proxy.go`
- `pkg/waveproxy/scheduler/scheduler.go`
- `pkg/waveproxy/rpc/rpc.go`
- `cmd/wsh/cmd/wshcmd-proxy.go`

### 2.4 MCP Server 管理（Claude/Codex/Gemini）

- 在设置界面新增 MCP Servers 管理。
- 支持 MCP server 的增删改查。
- 支持按应用启用/禁用（Claude Code / Codex CLI / Gemini CLI）。
- 支持从本机已安装应用导入并同步回对应配置文件。
- 支持 `stdio` / `http` / `sse` 传输类型。

代码落点：

- `frontend/app/view/waveconfig/mcpcontent.tsx`
- `pkg/mcpconfig/service.go`
- `pkg/mcpconfig/claude.go`
- `pkg/mcpconfig/codex.go`
- `pkg/mcpconfig/gemini.go`

### 2.5 窗口标题自动/固定模式

- 增加窗口标题自动模式（基于当前焦点上下文）。
- 增加窗口重命名（固定标题）与恢复自动模式。
- 菜单中提供直接入口。

代码落点：

- `frontend/app/window/windowtitle.tsx`
- `frontend/app/modals/renamewindowmodal.tsx`
- `emain/emain-menu.ts`

### 2.6 中英文双语 UI（English / 简体中文）

- 新增统一 i18n 层，支持英文与简体中文。
- 默认语言调整为英文（`app:language = en`）。
- 设置页切换语言即时生效（无需重启）。

代码落点：

- `frontend/app/i18n/i18n-core.ts`
- `frontend/app/i18n/i18n.ts`
- `frontend/app/view/waveconfig/settingscontent.tsx`

### 2.7 远程连接稳定性增强（SSH/WSL）

- 远程 `wsh` 安装路径统一为 `~/.tideterm/bin/wsh`。
- 连接和 shell 初始化中增强 HOME 推断逻辑（优先 passwd 信息），降低异常环境下路径解析错误。
- 改进远程安装/命令组装的健壮性与重试行为。
- 远程 `tmux` 自动续连能力默认开启（可在设置中关闭）。

代码落点：

- `pkg/remote/conncontroller/conncontroller.go`
- `pkg/remote/connutil.go`
- `pkg/shellexec/shellexec.go`
- `pkg/util/shellutil/shellintegration/bash_bashrc.sh`
- `pkg/util/shellutil/shellintegration/zsh_zshrc.sh`
- `pkg/util/shellutil/shellintegration/fish_wavefish.sh`
- `pkg/util/shellutil/shellintegration/pwsh_wavepwsh.sh`

## 3) 默认值与发行行为变化

- 品牌/标识改为 TideTerm（`name`、`productName`、`appId`）。
- 默认关闭遥测：`telemetry:enabled = false`。
- 默认关闭自动更新：`autoupdate:enabled = false`。
- 默认启用远程 tmux 续连：`term:remotetmuxresume = true`。
- 默认主页改为 TideTerm 仓库地址。
- 使用独立配置/数据目录与环境变量：`TIDETERM_*`。

相关文件：

- `package.json`
- `pkg/wconfig/defaultconfig/settings.json`
- `pkg/wavebase/wavebase.go`
- `electron-builder.config.cjs`

## 4) 兼容性与迁移提醒

- TideTerm 与上游 Wave 可共存（配置/数据命名空间独立）。
- 依赖旧路径的脚本需要迁移：
  - `~/.waveterm/...` → `~/.tideterm/...`
- 如果你在团队里启用了自动化部署/监控，请确认新 app 标识与产物命名。

## 5) 建议对外声明（可直接引用）

TideTerm is an independent fork of Wave Terminal (Apache-2.0).
This fork adds multi-session terminal blocks, built-in API Proxy (WaveProxy), MCP server management for Claude/Codex/Gemini, remote tmux session management, bilingual UI (English/简体中文), and several remote connection robustness improvements.

## 6) GitHub Release 可复制摘要

### 中文版本（简短）

- 新增单块多会话终端（会话侧栏 + 快速切换/结束）
- 新增远程 tmux 会话管理（Attach/Force Attach/Rename/Kill/Kill All）
- 新增内置 API Proxy（多通道、指标、历史、健康检查）
- 新增 MCP Servers 管理（Claude/Codex/Gemini 导入与同步）
- 增强 SSH/WSL 远程连接稳定性，统一远程安装路径为 `~/.tideterm`
- 默认值调整：遥测关闭、自动更新关闭、远程 tmux 续连开启

### English Version (Short)

- Added multi-session terminals within a single terminal block
- Added remote tmux session manager (Attach / Force Attach / Rename / Kill / Kill All)
- Added built-in API Proxy (WaveProxy) with channels, metrics, history, and health checks
- Added MCP server manager with import/sync for Claude Code / Codex CLI / Gemini CLI
- Improved SSH/WSL remote robustness and standardized helper path to `~/.tideterm`
- Changed defaults: telemetry off, auto-update off, remote tmux resume on

