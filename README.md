<p align="center">
  <a href="https://github.com/sanshao85/tideterm">
    <img alt="TideTerm Logo" src="./assets/appicon-source-1024.jpg" width="220">
  </a>
</p>

# TideTerm

English | [中文](./README.zh-CN.md)

TideTerm is a modern terminal app that combines traditional terminals with graphical “blocks” (files, previews, web, editor, AI chat). It runs on **macOS / Linux / Windows**.

This repository is a **fork of Wave Terminal** (Apache-2.0) by Command Line Inc. TideTerm is **not affiliated with** or endorsed by the upstream authors. See `MODIFICATIONS.md` for what changed in this fork.

For release-friendly fork delta notes (including copyable release summary text), see `RELEASE_NOTES_FORK_DIFF.md`.

## Table of Contents

- [Highlights](#highlights)
- [Fork Diff Release Notes](#fork-diff-release-notes)
- [Install](#install)
- [Getting Started](#getting-started)
- [Language (English / 中文)](#language-english--中文)
- [Remote Connections (SSH / WSL)](#remote-connections-ssh--wsl)
  - [wsh (Shell Extensions)](#wsh-shell-extensions)
  - [Remote Terminal Resume (tmux)](#remote-terminal-resume-tmux)
  - [Multi-Session Terminals (Single Block)](#multi-session-terminals-single-block)
  - [tmux Session Manager](#tmux-session-manager)
- [Drag & Drop Paths into Terminal](#drag--drop-paths-into-terminal)
- [Open Current Directory in a New Block](#open-current-directory-in-a-new-block)
- [Window Titles (Auto / Rename)](#window-titles-auto--rename)
- [MCP Server Manager](#mcp-server-manager)
- [API Proxy (WaveProxy)](#api-proxy-waveproxy)
- [Privacy Defaults](#privacy-defaults)
- [Config / Data Locations](#config--data-locations)
- [Build from Source](#build-from-source)
- [CI / Releases](#ci--releases)
- [Troubleshooting](#troubleshooting)
- [License](#license)

## Highlights

- **Block-based workspace**: terminals, files, previews, web, editor, AI chat
- **Remote connections** (SSH/WSL) with file browsing, preview, and remote editing
- **Command Blocks**: isolate a command into its own block for monitoring
- **`wsh` CLI**: control TideTerm workspace and move files between local/remote
- **Multi-session terminal blocks**: run/switch multiple terminals in one terminal block
- **Built-in MCP server manager**: import/sync for Claude Code / Codex CLI / Gemini CLI
- **API Proxy (WaveProxy)**: multi-channel AI proxy with metrics/history for Claude / Codex / Gemini clients
- **English + Simplified Chinese UI** with instant switching (no restart)

## Fork Diff Release Notes

- `RELEASE_NOTES_FORK_DIFF.md` contains a release-oriented summary of TideTerm vs upstream.
- It includes ready-to-copy Chinese and English bullet points for GitHub Releases.

## Install

- Download from GitHub Releases: `https://github.com/sanshao85/tideterm/releases`
## Getting Started

### Create blocks

- Use the sidebar to create blocks like **Terminal**, **Files**, **Web**, **Editor**, etc.
- You can drag blocks around to rearrange the workspace.

### Work with files

- In a **Files** block, you can browse folders and right-click for actions (open preview, open terminal, download, rename, etc.).
- For remote hosts, TideTerm can browse the remote filesystem and open a remote editor/preview in new blocks.

### Remote workflow basics

- Connect to a remote host via **SSH** (and **WSL** on Windows).
- TideTerm may prompt to install `wsh` on the remote machine (recommended). See details below.

## Language (English / 中文)

- TideTerm supports **English** and **Simplified Chinese (中文)** only.
- Default language is **English**.
- Switching language takes effect **immediately** (no restart): open **Settings** → **General** → **Language**.

## Remote Connections (SSH / WSL)

TideTerm can connect to remote machines and run terminals + browse/edit files there.

### wsh (Shell Extensions)

On first connect to a new SSH/WSL target, TideTerm can install `wsh` (a small helper) on the remote machine.

What `wsh` is used for:

- Enables features like remote **file browsing**, **file preview**, and **remote file actions**.
- Supplies metadata TideTerm uses for workflow features (for example, “open terminal in new block” in the right directory).

Where it is installed:

- Remote install directory is under your remote home directory: `~/.tideterm/bin/wsh`

If you choose **No wsh**:

- You can still open a plain remote shell, but some features will be unavailable or degraded.

Robustness details:

- TideTerm normalizes remote `HOME` (prefer passwd-resolved home) during `wsh` install and remote shell/tmux bootstrap, which avoids path/cwd issues in environments that override `HOME`.
- Transient `wsh` install/start failures are not permanently persisted as `conn:wshenabled=false`, so temporary network issues do not silently disable remote features.
- If a connection is already marked connected but the `conn:*` route is missing, TideTerm performs route self-heal (re-check/re-enable `wsh` and wait for route registration) before reporting failure.

### Remote Terminal Resume (tmux)

By default, TideTerm tries to make remote terminals resumable:

- Setting key: `term:remotetmuxresume` (default **true**)
- UI location: **Settings** → **General** → **Remote Terminal Resume**

Behavior:

- If `tmux` is available on the remote machine, TideTerm uses it so your session can resume after reconnect (network drop, sleep/wake, etc.).
- If `tmux` is not installed, TideTerm falls back to a normal shell and shows an install hint.

Install `tmux` (examples):

- Debian/Ubuntu: `sudo apt-get update && sudo apt-get install -y tmux`
- Fedora: `sudo dnf install -y tmux`
- RHEL/CentOS: `sudo yum install -y tmux` (or `dnf` on newer distros)
- Arch: `sudo pacman -S tmux`

### Multi-Session Terminals (Single Block)

TideTerm supports multiple terminal sessions inside one terminal block.

What you can do:

- Click **New Terminal** in the terminal toolbar to create another session in the same block
- Toggle **Show Terminal List / Hide Terminal List** to open the session sidebar
- Switch the active session from the sidebar
- Kill an individual session without closing the whole block
- Resize the session sidebar; width is persisted for the block

Behavior details:

- New sessions inherit connection + current directory from the active session when available
- If only one session remains, TideTerm can return to a normal single-terminal view

### tmux Session Manager

For remote connections with tmux, TideTerm includes a session manager to help you reattach or clean up sessions:

- Open the **server** icon in the remote terminal toolbar (Manage tmux sessions)
- View all tmux sessions for that connection with recent activity and “This Block” highlighting
- **Attach** / **Force Attach** / **Rename** / **Kill** sessions (including **Kill All**)

## Drag & Drop Paths into Terminal

TideTerm supports inserting paths into a terminal input by drag & drop.

Local terminal (macOS Finder → TideTerm local terminal):

- Drop one or more files/folders into a local terminal block.
- TideTerm inserts absolute paths like:
  - `/Users/admin/Desktop/kkk /Users/admin/Desktop/node-pty`
- Multiple items are space-separated.

Remote terminal (remote Files block → remote terminal block):

- Drop remote files/folders into a remote terminal block to insert remote paths.

## Open Current Directory in a New Block

When you right-click inside a terminal block, TideTerm can open the terminal’s current directory as a new Files block.

Notes:

- TideTerm uses shell/terminal metadata (OSC sequences) to track the terminal’s current working directory.
- In `tmux`, TideTerm relies on tmux OSC passthrough so the directory can still be detected.

## Window Titles (Auto / Rename)

TideTerm supports both automatic window titles and user-defined window names.

- Auto title: based on your current focus (for example, terminal directory context)
- Rename window: set a fixed title for a window (persisted)
- Restore auto title: switch back from fixed → auto

## MCP Server Manager

TideTerm includes an MCP server manager to configure MCP servers and sync them into supported AI apps.

Where:

- Open **Settings** → **MCP Servers**

What it can do:

- Create/edit/delete MCP server definitions
- Enable/disable a server per app:
  - Claude Code
  - Codex CLI
  - Gemini CLI
- Import MCP servers from installed apps
- Sync all enabled servers into the selected apps

Supported server transports:

- `stdio`
- `http`
- `sse`

Config targets:

- When syncing to apps, TideTerm updates config files for tools on the **local machine**, such as:
  - `~/.claude.json` (Claude Code)
  - `~/.codex/config.toml` (Codex CLI)
  - `~/.gemini/settings.json` (Gemini CLI)

## API Proxy (WaveProxy)

TideTerm includes a built-in AI API proxy (WaveProxy) for routing Claude / Codex / Gemini clients through a single endpoint, with multi-channel routing and observability.

Where:

- Create a new block → **API Proxy**

What it can do:

- Start/stop the proxy and set the listening port (default `3000`)
- Manage **channels** for `messages`, `responses`, and `gemini` endpoints, each with multiple API keys, base URLs, priority, and auth mode
- Configure proxy-level `accessKey` plus per-channel `authType` (`x-api-key`, `bearer`, `both`; Gemini also supports `x-goog-api-key`)
- Expose compatibility routes for OpenAI-style clients using either root or `.../v1` base URLs (`/v1/responses` + `/responses`, `/v1/models` + `/models`)
- Health check (“Ping”) channels and view **metrics** + **request history**
- Sync local channels to a remote connection’s proxy configuration
- Minimize to the dock (bottom-right) to keep a per-connection status indicator

Config storage:

- `~/.config/tideterm/waveproxy.json` (respects `TIDETERM_CONFIG_HOME`)

## Privacy Defaults

- Telemetry is disabled by default (`telemetry:enabled=false`). See `PRIVACY.md`.
- Auto-update is disabled by default (`autoupdate:enabled=false`).
- Cloud AI mode shortcuts are hidden by default (`waveai:showcloudmodes=false`).

## Config / Data Locations

TideTerm uses separate config/data locations and `TIDETERM_*` environment variables so it can coexist with Wave installations.

Environment variables:

- `TIDETERM_CONFIG_HOME` (override config directory)
- `TIDETERM_DATA_HOME` (override data directory)

Default locations (typical):

- macOS
  - Data: `~/Library/Application Support/tideterm/`
  - Config: `~/.config/tideterm/`
  - Logs (dev): `~/Library/Application Support/tideterm-dev/waveapp.log`
- Linux
  - Data: `~/.local/share/tideterm/`
  - Config: `~/.config/tideterm/`
  - Logs (dev): `~/.local/share/tideterm-dev/waveapp.log`
- Windows (typical)
  - Data: `%LOCALAPPDATA%\\tideterm\\`
  - Config: `%USERPROFILE%\\.config\\tideterm\\` (unless overridden)

API Proxy config:

- `~/.config/tideterm/waveproxy.json`

Remote helper directory:

- `~/.tideterm/` on the remote machine

## Build from Source

See `BUILD.md`.

Quick commands:

- Install deps: `task init`
- Run dev (hot reload): `task dev`
- Run standalone (no reload): `task start`
- Package (artifacts under `make/`): `task package`

## CI / Releases

- GitHub Actions builds TideTerm for macOS/Linux/Windows on tag pushes (`v*`) and creates a draft release.
- If you don't configure signing/notarization secrets, the workflow still builds unsigned artifacts.
- See `.github/workflows/build-helper.yml` and `RELEASES.md`.

## Troubleshooting

### macOS: “App is damaged” warning

If macOS says the app is damaged and can’t be opened, remove the quarantine attribute:

- `sudo xattr -dr com.apple.quarantine "/Applications/TideTerm.app"`

### Remote connection issues

- If remote features are missing, verify `wsh` is installed on the remote host and enabled for the connection.
- If you encounter route errors like `no route for "conn:..."`, reconnect once; TideTerm also includes auto route self-heal for connected sessions.
- If connserver exits with `137`/`SIGKILL`, check remote host memory/resource limits (for example `dmesg`) and restart the connection.
- You can also reinstall `wsh` for a connection using the local CLI:
  - `wsh conn status`
  - `wsh conn reinstall 'user@host:port'`

### Performance / rendering issues

- If you see terminal rendering problems, you can try disabling the WebGL renderer:
  - setting key: `term:disablewebgl`
- If the whole window has GPU-related issues, you can try:
  - setting key: `window:disablehardwareacceleration`

## License

TideTerm is licensed under the Apache-2.0 License (see `LICENSE`). Upstream notices are preserved in `NOTICE`.
