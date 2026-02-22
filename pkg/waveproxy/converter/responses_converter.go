// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package converter

import (
	"encoding/json"
	"fmt"
)

// ResponsesConverter converts between Responses API and Claude formats
type ResponsesConverter struct{}

// Responses API types (Codex-style)
type ResponsesRequest struct {
	Model              string          `json:"model"`
	Input              json.RawMessage `json:"input"`
	Instructions       string          `json:"instructions,omitempty"`
	PreviousResponseID string          `json:"previous_response_id,omitempty"`
	MaxOutputTokens    int             `json:"max_output_tokens,omitempty"`
	Stream             bool            `json:"stream"`
	Temperature        *float64        `json:"temperature,omitempty"`
}

type ResponsesResponse struct {
	ID     string                 `json:"id"`
	Object string                 `json:"object"`
	Output []ResponsesOutputItem  `json:"output"`
	Usage  ResponsesUsage         `json:"usage"`
}

type ResponsesOutputItem struct {
	Type    string                    `json:"type"`
	Role    string                    `json:"role,omitempty"`
	Content []ResponsesContentBlock   `json:"content,omitempty"`
}

type ResponsesContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ResponsesUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

func (c *ResponsesConverter) Name() string {
	return "responses"
}

// ToClaude converts Responses request to Claude format
func (c *ResponsesConverter) ToClaude(request []byte) ([]byte, error) {
	var respReq ResponsesRequest
	if err := json.Unmarshal(request, &respReq); err != nil {
		return nil, fmt.Errorf("failed to parse Responses request: %w", err)
	}

	// Parse input
	messages := c.parseInput(respReq.Input)

	claudeReq := ClaudeRequest{
		Model:    respReq.Model,
		Messages: messages,
		Stream:   respReq.Stream,
	}

	if respReq.MaxOutputTokens > 0 {
		claudeReq.MaxTokens = respReq.MaxOutputTokens
	} else {
		claudeReq.MaxTokens = 4096 // Default
	}

	if respReq.Temperature != nil {
		claudeReq.Temperature = respReq.Temperature
	}

	if respReq.Instructions != "" {
		claudeReq.System = respReq.Instructions
	}

	return json.Marshal(claudeReq)
}

// FromClaude converts Claude request to Responses format
func (c *ResponsesConverter) FromClaude(request []byte) ([]byte, error) {
	claudeReq, err := ParseClaudeRequest(request)
	if err != nil {
		return nil, err
	}

	// Convert messages to input
	var input []map[string]interface{}
	for _, msg := range claudeReq.Messages {
		content := ExtractTextFromContent(msg.Content)
		input = append(input, map[string]interface{}{
			"role":    msg.Role,
			"content": content,
		})
	}

	inputBytes, _ := json.Marshal(input)

	respReq := ResponsesRequest{
		Model:           claudeReq.Model,
		Input:           inputBytes,
		MaxOutputTokens: claudeReq.MaxTokens,
		Stream:          claudeReq.Stream,
		Temperature:     claudeReq.Temperature,
	}

	if claudeReq.System != nil {
		switch v := claudeReq.System.(type) {
		case string:
			respReq.Instructions = v
		}
	}

	return json.Marshal(respReq)
}

// ResponseToClaude converts Responses response to Claude format
func (c *ResponsesConverter) ResponseToClaude(response []byte) ([]byte, error) {
	var respResp ResponsesResponse
	if err := json.Unmarshal(response, &respResp); err != nil {
		return nil, fmt.Errorf("failed to parse Responses response: %w", err)
	}

	// Extract text from output
	var content string
	for _, item := range respResp.Output {
		if item.Type == "message" {
			for _, block := range item.Content {
				if block.Type == "output_text" || block.Type == "text" {
					content += block.Text
				}
			}
		}
	}

	claudeResp := ClaudeResponse{
		ID:         respResp.ID,
		Type:       "message",
		Role:       "assistant",
		StopReason: "end_turn",
		Content: []ClaudeContentBlock{
			{Type: "text", Text: content},
		},
		Usage: ClaudeUsage{
			InputTokens:  respResp.Usage.InputTokens,
			OutputTokens: respResp.Usage.OutputTokens,
		},
	}

	return json.Marshal(claudeResp)
}

// ResponseFromClaude converts Claude response to Responses format
func (c *ResponsesConverter) ResponseFromClaude(response []byte) ([]byte, error) {
	claudeResp, err := ParseClaudeResponse(response)
	if err != nil {
		return nil, err
	}

	var content string
	for _, block := range claudeResp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	respResp := ResponsesResponse{
		ID:     claudeResp.ID,
		Object: "response",
		Output: []ResponsesOutputItem{
			{
				Type: "message",
				Role: "assistant",
				Content: []ResponsesContentBlock{
					{Type: "output_text", Text: content},
				},
			},
		},
		Usage: ResponsesUsage{
			InputTokens:  claudeResp.Usage.InputTokens,
			OutputTokens: claudeResp.Usage.OutputTokens,
		},
	}

	return json.Marshal(respResp)
}

// parseInput parses the input field which can be string or array
func (c *ResponsesConverter) parseInput(input json.RawMessage) []ClaudeMessage {
	// Try as string
	var str string
	if err := json.Unmarshal(input, &str); err == nil {
		return []ClaudeMessage{
			{Role: "user", Content: BuildTextContent(str)},
		}
	}

	// Try as array of messages
	var messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(input, &messages); err == nil {
		result := make([]ClaudeMessage, 0, len(messages))
		for _, msg := range messages {
			content := ExtractTextFromContent(msg.Content)
			result = append(result, ClaudeMessage{
				Role:    msg.Role,
				Content: BuildTextContent(content),
			})
		}
		return result
	}

	// Fallback: treat as text
	return []ClaudeMessage{
		{Role: "user", Content: input},
	}
}
