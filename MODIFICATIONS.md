# TideTerm Modifications (Fork of Wave Terminal)

TideTerm is a fork of the open-source project **Wave Terminal** by **Command Line Inc.** (Apache-2.0). This file documents major changes made in this fork to satisfy Apache-2.0 redistribution requirements regarding modified works.

## Upstream

- Upstream project: Wave Terminal (`wavetermdev/waveterm`)
- Base upstream commit: `90011a7ede0931046f4c0843e9af027dcea8eefe` (2025-12-22)
- Fork repository: `sanshao85/tideterm`

TideTerm is **not affiliated with** and **not endorsed by** the upstream authors.

## Major Changes in TideTerm

- **Rebrand / Independent distribution**
  - App name and identifiers: `TideTerm`, `io.github.sanshao85.tideterm`
  - New application icon and various UI branding updates
  - GitHub links and release publishing targets updated to the TideTerm repository
- **Internationalization (EN/ZH)**
  - Added English + Simplified Chinese UI language support
  - Default language: English
  - Language switching applies immediately (no restart)
  - File panel context menus (including remote SSH/WSL extra entries like “Download File”) follow the selected language
- **Remote terminal behavior**
  - “Open Terminal in New Block” starts the shell in the expected directory instead of defaulting to the remote home directory
- **Drag & drop paths into terminal**
  - Improved drag/drop UX to insert paths into terminal input (local Finder → local terminal; remote file block → remote terminal)
  - Multiple items are inserted space-separated, with shell-safe quoting where appropriate
  - Focus handling improved so the terminal receives input focus after drop
- **Window title UX**
  - Added window renaming (manual “fixed title”) with persistence
  - Automatic title mode based on the currently focused block context
  - Fixed a performance issue that could cause UI stalls and React “maximum update depth” errors
- **MCP configuration management**
  - Added a built-in MCP server manager UI
  - Supports CRUD for MCP servers and app-specific enable/disable (Claude Code / Codex CLI / Gemini CLI)
  - Supports import/sync workflows and basic app install-status checks
- **Privacy / safety defaults**
  - Telemetry and auto-updater disabled by default
  - Cloud endpoints default to empty (no background calls unless explicitly configured)
- **Build/packaging stability**
  - Electron main bundle output switched to CommonJS to avoid a Vite ESM shim injection issue that could break bundling

## Compatibility Notes

- TideTerm uses separate config/data locations and environment variables (`TIDETERM_*`) to avoid conflicting with an existing Wave installation.
- Remote helper locations changed from `~/.waveterm/...` to `~/.tideterm/...`.

## License & Notices

- The upstream license is preserved in `LICENSE`.
- The upstream notice is preserved in `NOTICE`.
