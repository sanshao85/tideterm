# TideTerm

[English](./README.md) | 中文

TideTerm 是一款现代化终端应用，在传统终端的基础上，加入“图形化块（blocks）”能力（文件、预览、网页、编辑器、AI Chat 等）。支持 **macOS / Linux / Windows**。

本仓库是 **Wave Terminal**（Apache-2.0，Command Line Inc.）的 fork。TideTerm 与上游作者 **无关联**、也 **未获得背书**。fork 的改动说明请见 `MODIFICATIONS.md`。

如需查看更适合发布页使用的 fork 差异说明（含可直接复制的发布摘要），请见 `RELEASE_NOTES_FORK_DIFF.md`。

## 目录

- [功能亮点](#功能亮点)
- [Fork 差异发布说明](#fork-差异发布说明)
- [安装](#安装)
- [快速上手](#快速上手)
- [语言（English / 中文）](#语言english--中文)
- [远程连接（SSH / WSL）](#远程连接ssh--wsl)
  - [wsh（Shell 扩展）](#wshshell-扩展)
  - [远程终端续连（tmux）](#远程终端续连tmux)
  - [多会话终端（单个终端块）](#多会话终端单个终端块)
  - [tmux 会话管理](#tmux-会话管理)
- [拖拽路径到终端](#拖拽路径到终端)
- [在新块中打开当前目录](#在新块中打开当前目录)
- [窗口标题（自动 / 重命名）](#窗口标题自动--重命名)
- [MCP 服务器管理](#mcp-服务器管理)
- [API 代理（WaveProxy）](#api-代理waveproxy)
- [隐私默认值](#隐私默认值)
- [配置/数据目录](#配置数据目录)
- [从源码构建](#从源码构建)
- [CI / Releases](#ci--releases)
- [故障排查](#故障排查)
- [许可证](#许可证)

## 功能亮点

- **块式工作区**：终端、文件、预览、Web、编辑器、AI Chat
- **远程连接**（SSH/WSL）：远程文件浏览/预览/编辑
- **Command Blocks**：将单个命令隔离到独立块中运行与监控
- **`wsh` 命令行工具**：控制 TideTerm 工作区、在本地与远程之间传文件
- **多会话终端块**：在一个终端块里创建并切换多个终端会话
- **内置 MCP 服务器管理**：支持 Claude Code / Codex CLI / Gemini CLI 的导入与同步
- **内置 API 代理（WaveProxy）**：多通道路由与指标/历史，面向 Claude / Codex / Gemini 客户端
- **中英文双语**：即时切换（无需重启）

## Fork 差异发布说明

- `RELEASE_NOTES_FORK_DIFF.md` 提供了 TideTerm 相对上游的发布导向差异摘要。
- 文档内包含可直接复制到 GitHub Release 的中英文要点。

## 安装

- Releases（下载构建包）：`https://github.com/sanshao85/tideterm/releases`
## 快速上手

### 新建块

- 通过左侧栏创建 **Terminal** / **Files** / **Web** / **Editor** 等块。
- 你可以拖动块来重新布局。

### 文件工作流

- 在 **Files** 块里，可以浏览文件夹，右键可执行动作（预览/新块打开终端/下载/重命名等）。
- 连接远程后，Files 块也可以浏览远程文件系统，并在新块中打开远程预览/编辑。

### 远程使用要点

- 通过 **SSH**（Windows 上也支持 **WSL**）连接远程。
- 第一次连某个远程目标时，TideTerm 可能会提示安装 `wsh`（推荐）。详见下文。

## 语言（English / 中文）

- TideTerm 只提供 **English** 与 **简体中文（中文）** 两种语言。
- 默认语言：**English**。
- 切换语言 **立即生效**（无需重启）：打开 **Settings** → **General** → **Language**。

## 远程连接（SSH / WSL）

TideTerm 支持在远程主机上打开终端，并浏览/预览/编辑远程文件。

### wsh（Shell 扩展）

首次连接新的 SSH/WSL 目标时，TideTerm 可在远程主机上安装 `wsh`（一个小型辅助程序）。

`wsh` 的作用：

- 支持远程 **文件浏览**、**文件预览**、**远程文件右键动作** 等能力。
- 提供 TideTerm 所需的元信息（例如“在新块中打开终端时要从正确目录启动”）。

安装位置：

- 远程安装目录在远程用户家目录下：`~/.tideterm/bin/wsh`

如果你选择 **No wsh**：

- 仍然可以打开普通远程 shell，但部分功能会不可用或体验下降。

鲁棒性细节：

- 在 `wsh` 安装和远程 shell/tmux 启动流程中，TideTerm 会校正远程 `HOME`（优先使用 passwd 解析出的家目录），避免某些环境篡改 `HOME` 导致路径/cwd 异常。
- `wsh` 安装/启动的瞬时失败不会被永久写入为 `conn:wshenabled=false`，临时网络抖动不会悄悄把远程能力“锁死”。
- 如果连接状态已是 connected 但 `conn:*` 路由缺失，TideTerm 会先做一次自愈（重新检查/启用 `wsh` 并等待路由注册）再报错。

### 远程终端续连（tmux）

默认情况下 TideTerm 会尽量让远程终端可续连：

- 设置项 key：`term:remotetmuxresume`（默认 **true**）
- UI 位置：**Settings** → **General** → **Remote Terminal Resume**

行为说明：

- 如果远程主机安装了 `tmux`，TideTerm 会使用它，让断网/休眠重连后可以继续之前的会话。
- 如果远程没有安装 `tmux`，TideTerm 会退回普通 shell，并提示可选安装命令。

安装 `tmux`（示例）：

- Debian/Ubuntu：`sudo apt-get update && sudo apt-get install -y tmux`
- Fedora：`sudo dnf install -y tmux`
- RHEL/CentOS：`sudo yum install -y tmux`（新版本也可能是 `dnf`）
- Arch：`sudo pacman -S tmux`

### 多会话终端（单个终端块）

TideTerm 支持在一个终端块内运行多个终端会话。

你可以：

- 点击终端工具栏里的 **New Terminal** 在当前块新增会话
- 用 **Show Terminal List / Hide Terminal List** 打开或隐藏会话侧栏
- 在侧栏切换当前激活会话
- 单独结束某个会话，而不是关闭整个终端块
- 拖动调整会话侧栏宽度（会持久化）

行为说明：

- 新会话会尽量继承当前激活会话的连接与工作目录
- 当只剩一个会话时，可自动回到普通单终端视图

### tmux 会话管理

针对远程连接且启用 tmux 的场景，TideTerm 提供会话管理器，便于接入和清理会话：

- 入口：远程终端工具栏的 **服务器** 图标（管理 tmux 会话）
- 可查看该连接上的全部 tmux 会话与最近活跃时间，并标注“当前块”
- 支持 **接入** / **强制接入** / **重命名** / **结束**（以及 **全部结束**）

## 拖拽路径到终端

TideTerm 支持把文件/文件夹“拖进终端”，自动把路径插入到当前命令行输入。

本机终端（macOS Finder → TideTerm 本机终端）：

- 将一个或多个文件/文件夹拖进本机终端块。
- 会插入绝对路径，例如：
  - `/Users/admin/Desktop/kkk /Users/admin/Desktop/node-pty`
- 多个条目使用空格分隔。

远程终端（远程 Files 块 → 远程终端块）：

- 把远程文件/文件夹拖到远程终端块，会插入远程路径。

## 在新块中打开当前目录

在终端块内右键，TideTerm 支持“在新块中打开当前目录”（通常会新建一个 Files 块并定位到该目录）。

说明：

- TideTerm 依赖 shell/终端元信息（OSC 序列）来获取终端当前工作目录。
- 在 `tmux` 里，TideTerm 通过 tmux 的 OSC 透传机制来保证目录仍可被识别。

## 窗口标题（自动 / 重命名）

TideTerm 支持自动窗口标题，也支持手动为窗口命名（固定标题）。

- 自动标题：根据当前焦点上下文变化（例如终端所在目录）
- 重命名窗口：把窗口标题固定为你输入的名字（会持久化）
- 恢复自动标题：从“固定标题”切回“自动标题”

## MCP 服务器管理

TideTerm 内置 MCP 服务器管理界面，可统一管理 MCP servers，并同步到支持的 AI 工具中。

入口：

- 打开 **Settings** → **MCP Servers**

支持能力：

- 新增/编辑/删除 MCP 服务器
- 按应用启用/禁用服务器：
  - Claude Code
  - Codex CLI
  - Gemini CLI
- 从已安装应用导入 MCP 服务器
- 将已启用服务器一键同步到对应应用

支持的传输类型：

- `stdio`
- `http`
- `sse`

同步目标（配置写入位置）：

- 同步到应用时，TideTerm 会更新 **本机** 对应工具的配置文件，例如：
  - `~/.claude.json`（Claude Code）
  - `~/.codex/config.toml`（Codex CLI）
  - `~/.gemini/settings.json`（Gemini CLI）

## API 代理（WaveProxy）

TideTerm 内置 AI API 代理（WaveProxy），用于将 Claude / Codex / Gemini 等客户端统一接入，并提供多通道路由与可观测能力。

入口：

- 新建块 → **API 代理**

能力：

- 启动/停止代理并设置端口（默认 `3000`）
- 管理 `messages` / `responses` / `gemini` 三类通道，每个通道可配置多个 API Key、Base URL、优先级与鉴权模式
- 支持代理级 `accessKey`，以及通道级 `authType`（`x-api-key`、`bearer`、`both`；Gemini 额外支持 `x-goog-api-key`）
- 提供 OpenAI 风格兼容路由，兼容 `.../v1` 或根路径 Base URL 客户端（`/v1/responses` + `/responses`，`/v1/models` + `/models`）
- Ping 通道健康状况，查看 **指标** 与 **请求历史**
- 可将本地通道同步到远程连接的代理配置
- 最小化到右下角 Dock，显示各连接的代理状态，可点击恢复

配置存储：

- `~/.config/tideterm/waveproxy.json`（遵循 `TIDETERM_CONFIG_HOME`）

## 隐私默认值

- 默认不发送遥测数据（`telemetry:enabled=false`），详见 `PRIVACY.md`。
- 默认不启用自动更新（`autoupdate:enabled=false`）。
- 默认隐藏 Cloud AI 模式快捷入口（`waveai:showcloudmodes=false`）。

## 配置/数据目录

TideTerm 使用独立的配置/数据目录，以及 `TIDETERM_*` 环境变量，以便与 Wave 共存。

环境变量：

- `TIDETERM_CONFIG_HOME`（覆盖配置目录）
- `TIDETERM_DATA_HOME`（覆盖数据目录）

默认位置（常见情况）：

- macOS
  - Data：`~/Library/Application Support/tideterm/`
  - Config：`~/.config/tideterm/`
  - 日志（dev）：`~/Library/Application Support/tideterm-dev/waveapp.log`
- Linux
  - Data：`~/.local/share/tideterm/`
  - Config：`~/.config/tideterm/`
  - 日志（dev）：`~/.local/share/tideterm-dev/waveapp.log`
- Windows（常见情况）
  - Data：`%LOCALAPPDATA%\\tideterm\\`
  - Config：`%USERPROFILE%\\.config\\tideterm\\`（或自行覆盖）

API 代理配置：

- `~/.config/tideterm/waveproxy.json`

远程辅助目录：

- 远程主机上：`~/.tideterm/`

## 从源码构建

请参考 `BUILD.md`。

常用命令：

- 安装依赖：`task init`
- 开发模式（热更新）：`task dev`
- 直接运行（无热更新）：`task start`
- 打包（产物在 `make/`）：`task package`

## CI / Releases

- GitHub Actions 会在推送 tag（`v*`）时对 macOS/Linux/Windows 三平台进行构建，并创建一个 draft release。
- 如果没有配置签名/公证相关 secrets，也会构建出未签名的产物。
- 详见 `.github/workflows/build-helper.yml` 与 `RELEASES.md`。

## 故障排查

### macOS：提示“应用已损坏/无法打开”

如果 macOS 提示 TideTerm.app 已损坏或无法打开，可执行：

- `sudo xattr -dr com.apple.quarantine "/Applications/TideTerm.app"`

### 远程连接问题

- 远程功能缺失时，先确认远程目标已安装 `wsh`，且该连接启用了 `wsh`。
- 如果出现 `no route for "conn:..."` 这类路由错误，可先重连一次；TideTerm 对已连接会话也内置了路由自愈逻辑。
- 如果 connserver 以 `137`/`SIGKILL` 退出，优先检查远程主机内存/资源限制（例如 `dmesg`），然后重连。
- 也可以使用本机 CLI 重新安装某个连接的 `wsh`：
  - `wsh conn status`
  - `wsh conn reinstall 'user@host:port'`

### 性能/渲染问题

- 遇到终端渲染问题，可尝试禁用 WebGL 渲染器：
  - 设置 key：`term:disablewebgl`
- 整体窗口出现 GPU 相关问题，可尝试：
  - 设置 key：`window:disablehardwareacceleration`

## 许可证

TideTerm 使用 Apache-2.0 许可证（见 `LICENSE`）。上游 NOTICE 信息保留在 `NOTICE` 中。
