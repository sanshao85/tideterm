// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package mcpconfig

import (
	"os"
	"path/filepath"
	"runtime"
)

// getHomeDir returns the user's home directory
func getHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback
		if runtime.GOOS == "windows" {
			return os.Getenv("USERPROFILE")
		}
		return os.Getenv("HOME")
	}
	return home
}

// Claude Code paths

// GetClaudeConfigDir returns the Claude config directory path (~/.claude)
func GetClaudeConfigDir() string {
	return filepath.Join(getHomeDir(), ".claude")
}

// GetClaudeMcpPath returns the Claude MCP config file path (~/.claude.json)
func GetClaudeMcpPath() string {
	return filepath.Join(getHomeDir(), ".claude.json")
}

// Codex paths

// GetCodexConfigDir returns the Codex config directory path (~/.codex)
func GetCodexConfigDir() string {
	return filepath.Join(getHomeDir(), ".codex")
}

// GetCodexConfigPath returns the Codex config file path (~/.codex/config.toml)
func GetCodexConfigPath() string {
	return filepath.Join(GetCodexConfigDir(), "config.toml")
}

// Gemini paths

// GetGeminiConfigDir returns the Gemini config directory path (~/.gemini)
func GetGeminiConfigDir() string {
	return filepath.Join(getHomeDir(), ".gemini")
}

// GetGeminiSettingsPath returns the Gemini settings file path (~/.gemini/settings.json)
func GetGeminiSettingsPath() string {
	return filepath.Join(GetGeminiConfigDir(), "settings.json")
}

// AppInstalled checks if the given app's config directory exists
func AppInstalled(app AppType) bool {
	var dir string
	switch app {
	case AppClaude:
		// Claude uses ~/.claude.json or ~/.claude directory
		if _, err := os.Stat(GetClaudeMcpPath()); err == nil {
			return true
		}
		dir = GetClaudeConfigDir()
	case AppCodex:
		dir = GetCodexConfigDir()
	case AppGemini:
		dir = GetGeminiConfigDir()
	default:
		return false
	}

	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

// GetInstalledApps returns a list of installed apps
func GetInstalledApps() []AppType {
	var result []AppType
	for _, app := range AllApps() {
		if AppInstalled(app) {
			result = append(result, app)
		}
	}
	return result
}
