// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package mcpconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ClaudeConfig represents the ~/.claude.json file structure
type ClaudeConfig struct {
	McpServers            map[string]interface{} `json:"mcpServers,omitempty"`
	HasCompletedOnboarding bool                  `json:"hasCompletedOnboarding,omitempty"`
	// Other fields are preserved
	Extra map[string]interface{} `json:"-"`
}

// ReadClaudeConfig reads the Claude MCP configuration from ~/.claude.json
func ReadClaudeConfig() (*ClaudeConfig, error) {
	path := GetClaudeMcpPath()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &ClaudeConfig{
			McpServers: make(map[string]interface{}),
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	// First unmarshal to get all fields
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	config := &ClaudeConfig{
		McpServers: make(map[string]interface{}),
		Extra:      make(map[string]interface{}),
	}

	// Extract known fields
	if servers, ok := raw["mcpServers"].(map[string]interface{}); ok {
		config.McpServers = servers
	}
	if completed, ok := raw["hasCompletedOnboarding"].(bool); ok {
		config.HasCompletedOnboarding = completed
	}

	// Store other fields
	for k, v := range raw {
		if k != "mcpServers" && k != "hasCompletedOnboarding" {
			config.Extra[k] = v
		}
	}

	return config, nil
}

// WriteClaudeConfig writes the Claude MCP configuration to ~/.claude.json
func WriteClaudeConfig(config *ClaudeConfig) error {
	path := GetClaudeMcpPath()

	// Build the output map preserving extra fields
	output := make(map[string]interface{})
	for k, v := range config.Extra {
		output[k] = v
	}

	// Set known fields
	if len(config.McpServers) > 0 {
		output["mcpServers"] = config.McpServers
	}
	if config.HasCompletedOnboarding {
		output["hasCompletedOnboarding"] = true
	}

	// Marshal with indentation
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Atomic write
	return atomicWriteFile(path, data)
}

// ReadClaudeMcpServers reads the MCP servers from Claude config
func ReadClaudeMcpServers() (map[string]*McpServer, error) {
	config, err := ReadClaudeConfig()
	if err != nil {
		return nil, err
	}

	result := make(map[string]*McpServer)
	for id, rawSpec := range config.McpServers {
		server, err := parseClaudeServerSpec(id, rawSpec)
		if err != nil {
			// Log warning but continue
			fmt.Printf("Warning: skipping invalid MCP server '%s': %v\n", id, err)
			continue
		}
		result[id] = server
	}

	return result, nil
}

// parseClaudeServerSpec parses a Claude server spec into our unified format
func parseClaudeServerSpec(id string, rawSpec interface{}) (*McpServer, error) {
	specMap, ok := rawSpec.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("server spec must be an object")
	}

	spec := McpServerSpec{}

	// Parse type (default to stdio)
	if t, ok := specMap["type"].(string); ok {
		spec.Type = TransportType(t)
	} else {
		spec.Type = TransportStdio
	}

	// Parse stdio fields
	if cmd, ok := specMap["command"].(string); ok {
		spec.Command = cmd
	}
	if args, ok := specMap["args"].([]interface{}); ok {
		for _, arg := range args {
			if s, ok := arg.(string); ok {
				spec.Args = append(spec.Args, s)
			}
		}
	}
	if env, ok := specMap["env"].(map[string]interface{}); ok {
		spec.Env = make(map[string]string)
		for k, v := range env {
			if s, ok := v.(string); ok {
				spec.Env[k] = s
			}
		}
	}
	if cwd, ok := specMap["cwd"].(string); ok {
		spec.Cwd = cwd
	}

	// Parse http/sse fields
	if url, ok := specMap["url"].(string); ok {
		spec.URL = url
	}
	if headers, ok := specMap["headers"].(map[string]interface{}); ok {
		spec.Headers = make(map[string]string)
		for k, v := range headers {
			if s, ok := v.(string); ok {
				spec.Headers[k] = s
			}
		}
	}

	// Validate
	if err := spec.Validate(); err != nil {
		return nil, err
	}

	return &McpServer{
		ID:     id,
		Name:   id,
		Server: spec,
		Apps: McpApps{
			Claude: true,
		},
	}, nil
}

// WriteClaudeMcpServer writes a single MCP server to Claude config
func WriteClaudeMcpServer(server *McpServer) error {
	config, err := ReadClaudeConfig()
	if err != nil {
		return err
	}

	if config.McpServers == nil {
		config.McpServers = make(map[string]interface{})
	}

	// Convert server spec to Claude format
	specMap, err := serverSpecToClaudeFormat(&server.Server)
	if err != nil {
		return err
	}

	config.McpServers[server.ID] = specMap
	return WriteClaudeConfig(config)
}

// DeleteClaudeMcpServer removes a server from Claude config
func DeleteClaudeMcpServer(id string) error {
	config, err := ReadClaudeConfig()
	if err != nil {
		return err
	}

	if config.McpServers == nil {
		return nil
	}

	delete(config.McpServers, id)
	return WriteClaudeConfig(config)
}

// serverSpecToClaudeFormat converts a McpServerSpec to Claude's JSON format
func serverSpecToClaudeFormat(spec *McpServerSpec) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	transportType := spec.GetType()
	if transportType != TransportStdio {
		result["type"] = string(transportType)
	}

	switch transportType {
	case TransportStdio:
		if spec.Command != "" {
			result["command"] = spec.Command
		}
		if len(spec.Args) > 0 {
			result["args"] = spec.Args
		}
		if len(spec.Env) > 0 {
			result["env"] = spec.Env
		}
		if spec.Cwd != "" {
			result["cwd"] = spec.Cwd
		}
	case TransportHTTP, TransportSSE:
		result["type"] = string(transportType)
		if spec.URL != "" {
			result["url"] = spec.URL
		}
		if len(spec.Headers) > 0 {
			result["headers"] = spec.Headers
		}
	}

	return result, nil
}

// atomicWriteFile writes data to a file atomically
func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to temp file
	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Rename (atomic on most systems)
	if err := os.Rename(tmpFile, path); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}
