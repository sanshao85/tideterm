// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package mcpconfig

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// CodexConfig represents the ~/.codex/config.toml file structure
type CodexConfig struct {
	McpServers map[string]interface{} `toml:"mcp_servers,omitempty"`
	// Other fields are preserved
	Extra map[string]interface{} `toml:"-"`
}

// ReadCodexConfig reads the Codex MCP configuration from ~/.codex/config.toml
func ReadCodexConfig() (*CodexConfig, error) {
	path := GetCodexConfigPath()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &CodexConfig{
			McpServers: make(map[string]interface{}),
			Extra:      make(map[string]interface{}),
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return &CodexConfig{
			McpServers: make(map[string]interface{}),
			Extra:      make(map[string]interface{}),
		}, nil
	}

	// Unmarshal to get all fields
	var raw map[string]interface{}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	config := &CodexConfig{
		McpServers: make(map[string]interface{}),
		Extra:      make(map[string]interface{}),
	}

	// Extract mcp_servers (correct format)
	if servers, ok := raw["mcp_servers"].(map[string]interface{}); ok {
		config.McpServers = servers
	}

	// Also check for legacy mcp.servers format
	if mcp, ok := raw["mcp"].(map[string]interface{}); ok {
		if servers, ok := mcp["servers"].(map[string]interface{}); ok {
			// Merge legacy format into main servers
			for k, v := range servers {
				if _, exists := config.McpServers[k]; !exists {
					config.McpServers[k] = v
				}
			}
		}
	}

	// Store other fields
	for k, v := range raw {
		if k != "mcp_servers" && k != "mcp" {
			config.Extra[k] = v
		}
	}

	return config, nil
}

// WriteCodexConfig writes the Codex MCP configuration to ~/.codex/config.toml
func WriteCodexConfig(config *CodexConfig) error {
	path := GetCodexConfigPath()

	// Build the output map preserving extra fields
	output := make(map[string]interface{})
	for k, v := range config.Extra {
		output[k] = v
	}

	// Set mcp_servers (using correct format)
	if len(config.McpServers) > 0 {
		output["mcp_servers"] = config.McpServers
	}

	// Remove legacy mcp.servers if it exists
	delete(output, "mcp")

	// Marshal to TOML
	data, err := toml.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Atomic write
	return atomicWriteFile(path, data)
}

// ReadCodexMcpServers reads the MCP servers from Codex config
func ReadCodexMcpServers() (map[string]*McpServer, error) {
	config, err := ReadCodexConfig()
	if err != nil {
		return nil, err
	}

	result := make(map[string]*McpServer)
	for id, rawSpec := range config.McpServers {
		server, err := parseCodexServerSpec(id, rawSpec)
		if err != nil {
			// Log warning but continue
			fmt.Printf("Warning: skipping invalid Codex MCP server '%s': %v\n", id, err)
			continue
		}
		result[id] = server
	}

	return result, nil
}

// parseCodexServerSpec parses a Codex server spec (TOML) into our unified format
func parseCodexServerSpec(id string, rawSpec interface{}) (*McpServer, error) {
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
	// Codex uses http_headers (not headers)
	if headers, ok := specMap["http_headers"].(map[string]interface{}); ok {
		spec.Headers = make(map[string]string)
		for k, v := range headers {
			if s, ok := v.(string); ok {
				spec.Headers[k] = s
			}
		}
	}
	// Also check for legacy headers field
	if headers, ok := specMap["headers"].(map[string]interface{}); ok {
		if spec.Headers == nil {
			spec.Headers = make(map[string]string)
		}
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
			Codex: true,
		},
	}, nil
}

// WriteCodexMcpServer writes a single MCP server to Codex config
func WriteCodexMcpServer(server *McpServer) error {
	config, err := ReadCodexConfig()
	if err != nil {
		return err
	}

	if config.McpServers == nil {
		config.McpServers = make(map[string]interface{})
	}

	// Convert server spec to Codex TOML format
	specMap := serverSpecToCodexFormat(&server.Server)
	config.McpServers[server.ID] = specMap

	return WriteCodexConfig(config)
}

// DeleteCodexMcpServer removes a server from Codex config
func DeleteCodexMcpServer(id string) error {
	config, err := ReadCodexConfig()
	if err != nil {
		return err
	}

	if config.McpServers == nil {
		return nil
	}

	delete(config.McpServers, id)
	return WriteCodexConfig(config)
}

// serverSpecToCodexFormat converts a McpServerSpec to Codex's TOML format
func serverSpecToCodexFormat(spec *McpServerSpec) map[string]interface{} {
	result := make(map[string]interface{})

	transportType := spec.GetType()
	result["type"] = string(transportType)

	switch transportType {
	case TransportStdio:
		if spec.Command != "" {
			result["command"] = spec.Command
		}
		if len(spec.Args) > 0 {
			result["args"] = spec.Args
		}
		if len(spec.Env) > 0 {
			// Sort env keys for consistent output
			env := make(map[string]string)
			keys := make([]string, 0, len(spec.Env))
			for k := range spec.Env {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				env[k] = spec.Env[k]
			}
			result["env"] = env
		}
		if spec.Cwd != "" {
			result["cwd"] = spec.Cwd
		}
	case TransportHTTP, TransportSSE:
		if spec.URL != "" {
			result["url"] = spec.URL
		}
		// Codex uses http_headers (not headers)
		if len(spec.Headers) > 0 {
			headers := make(map[string]string)
			keys := make([]string, 0, len(spec.Headers))
			for k := range spec.Headers {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				headers[k] = spec.Headers[k]
			}
			result["http_headers"] = headers
		}
	}

	return result
}
