# TideTerm Modifications (Fork of Wave Terminal)

TideTerm is a fork of the open-source project **Wave Terminal** by **Command Line Inc.** (Apache-2.0).

This file documents notable changes made in this fork. It is intended to help downstream users understand what differs from upstream, and to satisfy Apache-2.0 redistribution requirements for modified works (while the upstream license/notice files remain the source of truth).

## Upstream

- Upstream project: Wave Terminal (`wavetermdev/waveterm`)
- Base upstream commit: `90011a7ede0931046f4c0843e9af027dcea8eefe` (2025-12-22)
- Fork repository: `sanshao85/tideterm`

TideTerm is **not affiliated with** and **not endorsed by** the upstream authors.

## Summary of Major Changes

### 1) Rebrand / independent distribution

- App name and identifiers changed to TideTerm:
  - Product name: `TideTerm`
  - App ID: `io.github.sanshao85.tideterm`
- New application icon and UI branding updates
- Repository links, homepage, and release publishing targets updated to the TideTerm repository
- Separate config/data namespaces and environment variables (`TIDETERM_*`) to avoid conflicts with Wave installations

### 2) Defaults changed (privacy + UX)

- Language defaults to English: `app:language = "en"`
- Telemetry is disabled by default: `telemetry:enabled = false`
- Auto-updater is disabled by default: `autoupdate:enabled = false`
- Remote tmux resume is enabled by default: `term:remotetmuxresume = true`
- Default homepage points to the TideTerm repo: `web:defaulturl = "https://github.com/sanshao85/tideterm"`

### 3) Internationalization (English / 简体中文)

- Added bilingual UI support: English + Simplified Chinese
- Default language: English
- Switching language applies immediately (no restart)
- Extended translation coverage to commonly missed areas:
  - Files panel context menus
  - Remote file menus with extra entries (for example “Download File”)
  - Selected workspace/terminal context menu strings

### 4) Remote connections (SSH / WSL)

#### wsh (shell extensions) install flow and robustness

- Improved first-connect experience with a clearer “Install wsh / No wsh” prompt
- Remote helper install location moved/standardized under the remote user’s home directory:
  - `~/.tideterm/bin/wsh`
- Improved robustness of remote install scripting by:
  - Avoiding reliance on potentially incorrect remote `$HOME` values
  - Resolving the remote user home from passwd data when available
  - Fixing quoting/command-assembly edge cases that can break remote `sh -c` execution

#### Correct remote working directory (CWD) behavior

- Fixed “Open Terminal in New Block” so it starts in the expected directory (instead of always defaulting to remote home)
- Added/extended “Open Current Directory in New Block” behavior to improve terminal ↔ files navigation

#### Remote terminal resume via tmux (optional but recommended)

- Added/extended an auto mode that uses `tmux` on remote terminals when available so sessions can resume after reconnects (network drop, sleep/wake, etc.)
- Fallback behavior when `tmux` is not installed:
  - Open a normal remote shell (non-resumable)
  - Show an install hint (user can choose to install `tmux` or disable the feature globally)

#### Shell integration and tmux OSC passthrough

- Updated shell integration scripts to ensure terminal OSC metadata (for example, current directory) is preserved through `tmux` by using tmux passthrough sequences.

### 5) Terminal UX improvements

- Drag & drop paths into terminal input:
  - Local Finder/Explorer → local terminal
  - Remote Files block → remote terminal
  - Multiple items inserted space-separated with shell-safe quoting where appropriate
  - Focus handling improved so the terminal receives input focus after drop
- Output coalescing/buffering improvements to reduce visible flicker for tools that rapidly rewrite status lines (especially over remote connections with packet fragmentation)
- Added/expanded settings to help with rendering/performance troubleshooting:
  - `term:disablewebgl`
  - `window:disablehardwareacceleration`

### 6) Window title / window switcher UX

- Added “rename window” (manual/fixed title) with persistence
- Added an “auto title” mode based on the currently focused context
- Fixed a performance issue that could cause UI stalls and React “maximum update depth” errors under rapid window/menu interactions

### 7) MCP configuration management (Claude / Codex / Gemini)

- Added a built-in MCP server manager UI (Settings → MCP Servers)
- Implemented a backend service + RPC endpoints for:
  - Listing / creating / updating / deleting MCP servers
  - Enabling/disabling servers per supported app (Claude Code / Codex CLI / Gemini CLI)
  - Importing from installed apps and syncing back into them
- Supports multiple MCP transport types:
  - `stdio`, `http`, `sse`

### 8) Build / CI stability

- Packaging/build stability fixes (including Electron/Vite main-process bundling output adjustments)
- GitHub Actions build workflow updated for TideTerm and for building across macOS/Linux/Windows on tag pushes

## Compatibility Notes

- TideTerm uses separate config/data locations and environment variables (`TIDETERM_*`) to avoid conflicting with an existing Wave installation.
- Remote helper locations changed from `~/.waveterm/...` to `~/.tideterm/...`.

## License & Notices

- The upstream license is preserved in `LICENSE`.
- The upstream notice is preserved in `NOTICE`.
