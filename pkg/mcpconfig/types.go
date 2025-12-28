// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Package mcpconfig provides MCP (Model Context Protocol) server configuration management
// for Claude Code, Codex CLI, and Gemini CLI.
package mcpconfig

import (
	"encoding/json"
	"fmt"
)

// TransportType represents the MCP server transport type
type TransportType string

const (
	TransportStdio TransportType = "stdio"
	TransportHTTP  TransportType = "http"
	TransportSSE   TransportType = "sse"
)

// AppType represents the CLI application type
type AppType string

const (
	AppClaude AppType = "claude"
	AppCodex  AppType = "codex"
	AppGemini AppType = "gemini"
)

// AllApps returns all supported app types
func AllApps() []AppType {
	return []AppType{AppClaude, AppCodex, AppGemini}
}

// McpApps represents which applications a server is enabled for
type McpApps struct {
	Claude bool `json:"claude"`
	Codex  bool `json:"codex"`
	Gemini bool `json:"gemini"`
}

// IsEnabledFor checks if the server is enabled for the given app
func (a *McpApps) IsEnabledFor(app AppType) bool {
	switch app {
	case AppClaude:
		return a.Claude
	case AppCodex:
		return a.Codex
	case AppGemini:
		return a.Gemini
	}
	return false
}

// SetEnabledFor sets the enabled state for the given app
func (a *McpApps) SetEnabledFor(app AppType, enabled bool) {
	switch app {
	case AppClaude:
		a.Claude = enabled
	case AppCodex:
		a.Codex = enabled
	case AppGemini:
		a.Gemini = enabled
	}
}

// EnabledApps returns a list of apps that are enabled
func (a *McpApps) EnabledApps() []AppType {
	var result []AppType
	if a.Claude {
		result = append(result, AppClaude)
	}
	if a.Codex {
		result = append(result, AppCodex)
	}
	if a.Gemini {
		result = append(result, AppGemini)
	}
	return result
}

// McpServerSpec represents the server connection specification
type McpServerSpec struct {
	// Type is the transport type: stdio, http, or sse
	Type TransportType `json:"type,omitempty"`

	// stdio fields
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Cwd     string            `json:"cwd,omitempty"`

	// http/sse fields
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// GetType returns the transport type, defaulting to stdio if not set
func (s *McpServerSpec) GetType() TransportType {
	if s.Type == "" {
		return TransportStdio
	}
	return s.Type
}

// ToJSON converts the spec to a generic JSON map
func (s *McpServerSpec) ToJSON() (map[string]interface{}, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	// Remove empty fields
	for k, v := range result {
		if v == nil || v == "" {
			delete(result, k)
		}
		if arr, ok := v.([]interface{}); ok && len(arr) == 0 {
			delete(result, k)
		}
		if m, ok := v.(map[string]interface{}); ok && len(m) == 0 {
			delete(result, k)
		}
	}
	return result, nil
}

// McpServer represents a complete MCP server configuration
type McpServer struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Server      McpServerSpec  `json:"server"`
	Apps        McpApps        `json:"apps"`
	Description string         `json:"description,omitempty"`
	Homepage    string         `json:"homepage,omitempty"`
	Docs        string         `json:"docs,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
}

// McpConfigFile represents the MCP configuration file structure
type McpConfigFile struct {
	Servers map[string]*McpServer `json:"servers"`
}

// NewMcpConfigFile creates a new empty MCP config file
func NewMcpConfigFile() *McpConfigFile {
	return &McpConfigFile{
		Servers: make(map[string]*McpServer),
	}
}

// Validate validates the MCP server spec
func (s *McpServerSpec) Validate() error {
	transportType := s.GetType()

	switch transportType {
	case TransportStdio:
		if s.Command == "" {
			return fmt.Errorf("stdio type MCP server requires 'command' field")
		}
	case TransportHTTP, TransportSSE:
		if s.URL == "" {
			return fmt.Errorf("%s type MCP server requires 'url' field", transportType)
		}
	default:
		return fmt.Errorf("invalid transport type: %s (must be stdio, http, or sse)", transportType)
	}

	return nil
}

// Validate validates the MCP server
func (s *McpServer) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("server ID cannot be empty")
	}
	if s.Name == "" {
		s.Name = s.ID
	}
	return s.Server.Validate()
}

// ImportResult represents the result of an import operation
type ImportResult struct {
	Imported int      `json:"imported"`
	Errors   []string `json:"errors,omitempty"`
}
