// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package mcpconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/sanshao85/tideterm/pkg/wavebase"
)

const McpConfigFileName = "mcp.json"

var (
	configMutex sync.RWMutex
)

// GetMcpConfigPath returns the path to the MCP config file
func GetMcpConfigPath() string {
	return filepath.Join(wavebase.GetWaveConfigDir(), McpConfigFileName)
}

// ReadMcpConfig reads the MCP configuration from the config file
func ReadMcpConfig() (*McpConfigFile, error) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	path := GetMcpConfigPath()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return NewMcpConfigFile(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read MCP config: %w", err)
	}

	var config McpConfigFile
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse MCP config: %w", err)
	}

	if config.Servers == nil {
		config.Servers = make(map[string]*McpServer)
	}

	return &config, nil
}

// WriteMcpConfig writes the MCP configuration to the config file
func WriteMcpConfig(config *McpConfigFile) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	path := GetMcpConfigPath()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal MCP config: %w", err)
	}

	return atomicWriteFile(path, data)
}

// GetAllServers returns all MCP servers from the config
func GetAllServers() (map[string]*McpServer, error) {
	config, err := ReadMcpConfig()
	if err != nil {
		return nil, err
	}
	return config.Servers, nil
}

// GetServer returns a single MCP server by ID
func GetServer(id string) (*McpServer, error) {
	servers, err := GetAllServers()
	if err != nil {
		return nil, err
	}
	return servers[id], nil
}

// UpsertServer adds or updates an MCP server
func UpsertServer(server *McpServer) error {
	if err := server.Validate(); err != nil {
		return err
	}

	config, err := ReadMcpConfig()
	if err != nil {
		return err
	}

	// Get previous apps state for detecting disabled apps
	var prevApps McpApps
	if existing, ok := config.Servers[server.ID]; ok {
		prevApps = existing.Apps
	}

	// Save to config
	config.Servers[server.ID] = server
	if err := WriteMcpConfig(config); err != nil {
		return err
	}

	// Handle disabled apps - remove from live config
	if prevApps.Claude && !server.Apps.Claude {
		if AppInstalled(AppClaude) {
			DeleteClaudeMcpServer(server.ID)
		}
	}
	if prevApps.Codex && !server.Apps.Codex {
		if AppInstalled(AppCodex) {
			DeleteCodexMcpServer(server.ID)
		}
	}
	if prevApps.Gemini && !server.Apps.Gemini {
		if AppInstalled(AppGemini) {
			DeleteGeminiMcpServer(server.ID)
		}
	}

	// Sync to enabled apps
	return SyncServerToApps(server)
}

// DeleteServer removes an MCP server
func DeleteServer(id string) error {
	config, err := ReadMcpConfig()
	if err != nil {
		return err
	}

	server, ok := config.Servers[id]
	if !ok {
		return nil // Already doesn't exist
	}

	// Remove from all enabled apps
	for _, app := range server.Apps.EnabledApps() {
		if AppInstalled(app) {
			switch app {
			case AppClaude:
				DeleteClaudeMcpServer(id)
			case AppCodex:
				DeleteCodexMcpServer(id)
			case AppGemini:
				DeleteGeminiMcpServer(id)
			}
		}
	}

	// Remove from config
	delete(config.Servers, id)
	return WriteMcpConfig(config)
}

// ToggleApp enables or disables an app for a server
func ToggleApp(serverId string, app AppType, enabled bool) error {
	config, err := ReadMcpConfig()
	if err != nil {
		return err
	}

	server, ok := config.Servers[serverId]
	if !ok {
		return fmt.Errorf("server not found: %s", serverId)
	}

	server.Apps.SetEnabledFor(app, enabled)

	if err := WriteMcpConfig(config); err != nil {
		return err
	}

	// Sync to the affected app
	if enabled {
		return SyncServerToApp(server, app)
	} else {
		return RemoveServerFromApp(serverId, app)
	}
}

// SyncServerToApps syncs a server to all its enabled apps
func SyncServerToApps(server *McpServer) error {
	var errs []error

	for _, app := range server.Apps.EnabledApps() {
		if err := SyncServerToApp(server, app); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", app, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("sync errors: %v", errs)
	}
	return nil
}

// SyncServerToApp syncs a server to a specific app
func SyncServerToApp(server *McpServer, app AppType) error {
	if !AppInstalled(app) {
		return nil // Skip if app not installed
	}

	switch app {
	case AppClaude:
		return WriteClaudeMcpServer(server)
	case AppCodex:
		return WriteCodexMcpServer(server)
	case AppGemini:
		return WriteGeminiMcpServer(server)
	}
	return nil
}

// RemoveServerFromApp removes a server from a specific app
func RemoveServerFromApp(serverId string, app AppType) error {
	if !AppInstalled(app) {
		return nil
	}

	switch app {
	case AppClaude:
		return DeleteClaudeMcpServer(serverId)
	case AppCodex:
		return DeleteCodexMcpServer(serverId)
	case AppGemini:
		return DeleteGeminiMcpServer(serverId)
	}
	return nil
}

// SyncAllEnabled syncs all enabled servers to their respective apps
func SyncAllEnabled() error {
	servers, err := GetAllServers()
	if err != nil {
		return err
	}

	var errs []error
	for _, server := range servers {
		if err := SyncServerToApps(server); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", server.ID, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("sync errors: %v", errs)
	}
	return nil
}

// ImportFromApp imports MCP servers from a specific app
func ImportFromApp(app AppType) (*ImportResult, error) {
	if !AppInstalled(app) {
		return &ImportResult{}, nil
	}

	var servers map[string]*McpServer
	var err error

	switch app {
	case AppClaude:
		servers, err = ReadClaudeMcpServers()
	case AppCodex:
		servers, err = ReadCodexMcpServers()
	case AppGemini:
		servers, err = ReadGeminiMcpServers()
	default:
		return nil, fmt.Errorf("unknown app: %s", app)
	}

	if err != nil {
		return nil, err
	}

	config, err := ReadMcpConfig()
	if err != nil {
		return nil, err
	}

	result := &ImportResult{}

	for id, imported := range servers {
		if existing, ok := config.Servers[id]; ok {
			// Already exists - just enable this app
			existing.Apps.SetEnabledFor(app, true)
		} else {
			// New server
			config.Servers[id] = imported
			result.Imported++
		}
	}

	if err := WriteMcpConfig(config); err != nil {
		return nil, err
	}

	// Sync imported servers
	for id := range servers {
		if server, ok := config.Servers[id]; ok {
			SyncServerToApps(server)
		}
	}

	return result, nil
}

// ImportFromAllApps imports MCP servers from all installed apps
func ImportFromAllApps() (*ImportResult, error) {
	result := &ImportResult{}

	for _, app := range GetInstalledApps() {
		appResult, err := ImportFromApp(app)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", app, err))
			continue
		}
		result.Imported += appResult.Imported
	}

	return result, nil
}

// GetServersSorted returns all servers sorted by ID
func GetServersSorted() ([]*McpServer, error) {
	servers, err := GetAllServers()
	if err != nil {
		return nil, err
	}

	// Sort by ID
	ids := make([]string, 0, len(servers))
	for id := range servers {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	result := make([]*McpServer, 0, len(ids))
	for _, id := range ids {
		result = append(result, servers[id])
	}

	return result, nil
}

// GetAppStatus returns the installation status of all apps
func GetAppStatus() map[AppType]bool {
	result := make(map[AppType]bool)
	for _, app := range AllApps() {
		result[app] = AppInstalled(app)
	}
	return result
}
