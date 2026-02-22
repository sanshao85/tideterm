// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sanshao85/tideterm/pkg/waveproxy/config"
)

// ClaudeProvider handles Claude API requests
type ClaudeProvider struct {
	BaseProvider
}

// ClaudeRequest represents a Claude Messages API request
type ClaudeRequest struct {
	Model         string          `json:"model"`
	MaxTokens     int             `json:"max_tokens"`
	Messages      json.RawMessage `json:"messages"`
	System        json.RawMessage `json:"system,omitempty"`
	Stream        bool            `json:"stream"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	TopK          *int            `json:"top_k,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

// ClaudeResponse represents a Claude Messages API response
type ClaudeResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        ClaudeUsage    `json:"usage"`
}

// ContentBlock represents a content block in Claude response
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ClaudeUsage represents token usage in Claude response
type ClaudeUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens,omitempty"`
}

// Name returns the provider name
func (p *ClaudeProvider) Name() string {
	return "claude"
}

// SendRequest sends a request to Claude API
func (p *ClaudeProvider) SendRequest(ctx context.Context, ch *config.Channel, apiKey string, request []byte, stream bool) (*Response, error) {
	baseURLs := ch.GetAllBaseURLs()
	if len(baseURLs) == 0 {
		return nil, fmt.Errorf("no base URL configured")
	}

	url := baseURLs[0] + "/v1/messages"

	headers := map[string]string{
		"Content-Type":      "application/json",
		"x-api-key":         apiKey,
		"anthropic-version": "2023-06-01",
	}

	resp, err := p.DoRequest(ctx, "POST", url, headers, request)
	if err != nil {
		return nil, err
	}

	if stream {
		return &Response{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Stream:     resp.Body,
		}, nil
	}

	body, err := p.ReadResponse(resp)
	if err != nil {
		return nil, err
	}

	response := &Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       body,
	}

	// Parse usage if successful
	if resp.StatusCode == 200 {
		var claudeResp ClaudeResponse
		if err := json.Unmarshal(body, &claudeResp); err == nil {
			response.InputTokens = claudeResp.Usage.InputTokens
			response.OutputTokens = claudeResp.Usage.OutputTokens
			response.CacheRead = claudeResp.Usage.CacheReadInputTokens
			response.CacheCreate = claudeResp.Usage.CacheCreationInputTokens
		}
	}

	return response, nil
}

// ConvertRequest for Claude is a pass-through (native format)
func (p *ClaudeProvider) ConvertRequest(request []byte, model string) ([]byte, error) {
	var req map[string]interface{}
	if err := json.Unmarshal(request, &req); err != nil {
		return nil, err
	}
	req["model"] = model
	return json.Marshal(req)
}

// ConvertResponse for Claude is a pass-through (native format)
func (p *ClaudeProvider) ConvertResponse(response []byte) ([]byte, error) {
	return response, nil
}

// ParseStreamEvent parses a Claude streaming event
func (p *ClaudeProvider) ParseStreamEvent(event []byte) (*StreamEvent, error) {
	// Claude SSE format: "event: xxx\ndata: {...}\n\n"
	lines := strings.Split(string(event), "\n")

	var eventType string
	var data []byte

	for _, line := range lines {
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data = []byte(strings.TrimPrefix(line, "data: "))
		}
	}

	if eventType == "" && len(data) == 0 {
		return nil, nil // Empty event
	}

	streamEvent := &StreamEvent{
		Type: eventType,
		Data: data,
	}

	// Parse specific event types
	switch eventType {
	case "content_block_delta":
		var delta struct {
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(data, &delta); err == nil {
			streamEvent.Delta = delta.Delta.Text
		}
	case "message_delta":
		var delta struct {
			Usage struct {
				OutputTokens int64 `json:"output_tokens"`
			} `json:"usage"`
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(data, &delta); err == nil {
			streamEvent.OutputTokens = delta.Usage.OutputTokens
			streamEvent.StopReason = delta.Delta.StopReason
		}
	case "message_start":
		var start struct {
			Message struct {
				Usage struct {
					InputTokens int64 `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(data, &start); err == nil {
			streamEvent.InputTokens = start.Message.Usage.InputTokens
		}
	}

	return streamEvent, nil
}
