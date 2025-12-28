// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package mcpconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// GeminiConfig represents the ~/.gemini/settings.json file structure
type GeminiConfig struct {
	McpServers map[string]interface{} `json:"mcpServers,omitempty"`
	Security   interface{}            `json:"security,omitempty"`
	// Other fields are preserved
	Extra map[string]interface{} `json:"-"`
}

// ReadGeminiConfig reads the Gemini MCP configuration from ~/.gemini/settings.json
func ReadGeminiConfig() (*GeminiConfig, error) {
	path := GetGeminiSettingsPath()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &GeminiConfig{
			McpServers: make(map[string]interface{}),
			Extra:      make(map[string]interface{}),
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

	config := &GeminiConfig{
		McpServers: make(map[string]interface{}),
		Extra:      make(map[string]interface{}),
	}

	// Extract known fields
	if servers, ok := raw["mcpServers"].(map[string]interface{}); ok {
		config.McpServers = servers
	}
	if security, ok := raw["security"]; ok {
		config.Security = security
	}

	// Store other fields
	for k, v := range raw {
		if k != "mcpServers" && k != "security" {
			config.Extra[k] = v
		}
	}

	return config, nil
}

// WriteGeminiConfig writes the Gemini MCP configuration to ~/.gemini/settings.json
func WriteGeminiConfig(config *GeminiConfig) error {
	path := GetGeminiSettingsPath()

	// Build the output map preserving extra fields
	output := make(map[string]interface{})
	for k, v := range config.Extra {
		output[k] = v
	}

	// Set known fields
	if len(config.McpServers) > 0 {
		output["mcpServers"] = config.McpServers
	}
	if config.Security != nil {
		output["security"] = config.Security
	}

	// Marshal with indentation
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Atomic write
	return atomicWriteFile(path, data)
}

// ReadGeminiMcpServers reads the MCP servers from Gemini config
func ReadGeminiMcpServers() (map[string]*McpServer, error) {
	config, err := ReadGeminiConfig()
	if err != nil {
		return nil, err
	}

	result := make(map[string]*McpServer)
	for id, rawSpec := range config.McpServers {
		server, err := parseGeminiServerSpec(id, rawSpec)
		if err != nil {
			// Log warning but continue
			fmt.Printf("Warning: skipping invalid Gemini MCP server '%s': %v\n", id, err)
			continue
		}
		result[id] = server
	}

	return result, nil
}

// parseGeminiServerSpec parses a Gemini server spec into our unified format
// Gemini has special format: no "type" field, uses "httpUrl" for HTTP
func parseGeminiServerSpec(id string, rawSpec interface{}) (*McpServer, error) {
	specMap, ok := rawSpec.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("server spec must be an object")
	}

	spec := McpServerSpec{}

	// Gemini doesn't use type field - infer from fields present
	// - Has "command" -> stdio
	// - Has "httpUrl" -> http
	// - Has "url" -> sse
	if _, hasCommand := specMap["command"]; hasCommand {
		spec.Type = TransportStdio
	} else if _, hasHttpUrl := specMap["httpUrl"]; hasHttpUrl {
		spec.Type = TransportHTTP
	} else if _, hasUrl := specMap["url"]; hasUrl {
		spec.Type = TransportSSE
	} else {
		spec.Type = TransportStdio // default
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
	// Gemini uses "httpUrl" for HTTP, "url" for SSE
	if httpUrl, ok := specMap["httpUrl"].(string); ok {
		spec.URL = httpUrl
	}
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
			Gemini: true,
		},
	}, nil
}

// WriteGeminiMcpServer writes a single MCP server to Gemini config
func WriteGeminiMcpServer(server *McpServer) error {
	config, err := ReadGeminiConfig()
	if err != nil {
		return err
	}

	if config.McpServers == nil {
		config.McpServers = make(map[string]interface{})
	}

	// Convert server spec to Gemini format
	specMap := serverSpecToGeminiFormat(&server.Server)
	config.McpServers[server.ID] = specMap

	return WriteGeminiConfig(config)
}

// DeleteGeminiMcpServer removes a server from Gemini config
func DeleteGeminiMcpServer(id string) error {
	config, err := ReadGeminiConfig()
	if err != nil {
		return err
	}

	if config.McpServers == nil {
		return nil
	}

	delete(config.McpServers, id)
	return WriteGeminiConfig(config)
}

// serverSpecToGeminiFormat converts a McpServerSpec to Gemini's JSON format
// Gemini doesn't use "type" field and uses "httpUrl" for HTTP
func serverSpecToGeminiFormat(spec *McpServerSpec) map[string]interface{} {
	result := make(map[string]interface{})

	transportType := spec.GetType()
	// Note: Gemini doesn't use "type" field

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
	case TransportHTTP:
		// Gemini uses "httpUrl" for HTTP type
		if spec.URL != "" {
			result["httpUrl"] = spec.URL
		}
		if len(spec.Headers) > 0 {
			result["headers"] = spec.Headers
		}
	case TransportSSE:
		// Gemini uses "url" for SSE type
		if spec.URL != "" {
			result["url"] = spec.URL
		}
		if len(spec.Headers) > 0 {
			result["headers"] = spec.Headers
		}
	}

	return result
}
