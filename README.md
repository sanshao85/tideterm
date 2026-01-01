<p align="center">
  <a href="https://github.com/sanshao85/tideterm">
    <img alt="TideTerm Logo" src="./assets/appicon-source-1024.jpg" width="220">
  </a>
</p>

# TideTerm

English | [中文](./README.zh-CN.md)

TideTerm is a modern terminal app that combines traditional terminal features with graphical blocks such as file previews, web browsing, an editor, and AI chat. It runs on macOS, Linux, and Windows.

This repository is a **fork of Wave Terminal** (Apache-2.0) by Command Line Inc. See `MODIFICATIONS.md` for a summary of fork-specific changes.

## Highlights

- Drag & drop blocks: terminals, previews, web, editor, AI chat
- Remote connections (SSH/WSL) with file browsing and previews
- Built-in editor for remote files
- Command Blocks to isolate and monitor commands
- `wsh` CLI to control the workspace and move files between local/remote
- Built-in MCP server manager (import/sync for Claude Code, Codex CLI, Gemini CLI)
- English + Simplified Chinese UI with instant switching (no restart)

## Install

- Releases: https://github.com/sanshao85/tideterm/releases

- - 苹果系统如果遇到，文件损坏，请执行以下命令：
 sudo xattr -dr com.apple.quarantine "/Applications/TideTerm.app"

## Build from source

See `BUILD.md`.

## License

TideTerm is licensed under the Apache-2.0 License (see `LICENSE`). Upstream notices are preserved in `NOTICE`.
