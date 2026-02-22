// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Package converter provides protocol conversion between different AI service formats.
package converter

import (
	"encoding/json"
	"fmt"
)

// Converter interface for protocol conversion
type Converter interface {
	// Name returns the converter name
	Name() string

	// ToClaude converts request to Claude format
	ToClaude(request []byte) ([]byte, error)

	// FromClaude converts Claude format to target format
	FromClaude(request []byte) ([]byte, error)

	// ResponseToClaude converts response to Claude format
	ResponseToClaude(response []byte) ([]byte, error)

	// ResponseFromClaude converts Claude response to target format
	ResponseFromClaude(response []byte) ([]byte, error)
}

// GetConverter returns a converter for the specified format
func GetConverter(format string) Converter {
	switch format {
	case "openai":
		return &OpenAIConverter{}
	case "gemini":
		return &GeminiConverter{}
	case "responses":
		return &ResponsesConverter{}
	default:
		return &PassthroughConverter{}
	}
}

// PassthroughConverter passes data through without conversion
type PassthroughConverter struct{}

func (c *PassthroughConverter) Name() string                            { return "passthrough" }
func (c *PassthroughConverter) ToClaude(request []byte) ([]byte, error) { return request, nil }
func (c *PassthroughConverter) FromClaude(request []byte) ([]byte, error) { return request, nil }
func (c *PassthroughConverter) ResponseToClaude(response []byte) ([]byte, error) { return response, nil }
func (c *PassthroughConverter) ResponseFromClaude(response []byte) ([]byte, error) { return response, nil }

// ClaudeMessage represents a message in Claude format
type ClaudeMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// ClaudeRequest represents a Claude Messages API request
type ClaudeRequest struct {
	Model         string          `json:"model"`
	MaxTokens     int             `json:"max_tokens"`
	Messages      []ClaudeMessage `json:"messages"`
	System        interface{}     `json:"system,omitempty"`
	Stream        bool            `json:"stream"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	TopK          *int            `json:"top_k,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

// ClaudeResponse represents a Claude Messages API response
type ClaudeResponse struct {
	ID           string              `json:"id"`
	Type         string              `json:"type"`
	Role         string              `json:"role"`
	Content      []ClaudeContentBlock `json:"content"`
	Model        string              `json:"model"`
	StopReason   string              `json:"stop_reason"`
	StopSequence string              `json:"stop_sequence,omitempty"`
	Usage        ClaudeUsage         `json:"usage"`
}

// ClaudeContentBlock represents a content block
type ClaudeContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ClaudeUsage represents token usage
type ClaudeUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens,omitempty"`
}

// ExtractTextFromContent extracts text from Claude content format
func ExtractTextFromContent(content json.RawMessage) string {
	// Try as string
	var str string
	if err := json.Unmarshal(content, &str); err == nil {
		return str
	}

	// Try as array of content blocks
	var blocks []ClaudeContentBlock
	if err := json.Unmarshal(content, &blocks); err == nil {
		var result string
		for _, block := range blocks {
			if block.Type == "text" {
				result += block.Text
			}
		}
		return result
	}

	return string(content)
}

// BuildTextContent builds Claude content from text
func BuildTextContent(text string) json.RawMessage {
	content, _ := json.Marshal([]ClaudeContentBlock{{Type: "text", Text: text}})
	return content
}

// ParseClaudeRequest parses a Claude request
func ParseClaudeRequest(data []byte) (*ClaudeRequest, error) {
	var req ClaudeRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("failed to parse Claude request: %w", err)
	}
	return &req, nil
}

// ParseClaudeResponse parses a Claude response
func ParseClaudeResponse(data []byte) (*ClaudeResponse, error) {
	var resp ClaudeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Claude response: %w", err)
	}
	return &resp, nil
}
