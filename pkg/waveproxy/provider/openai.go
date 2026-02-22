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

// OpenAIProvider handles OpenAI API requests
type OpenAIProvider struct {
	BaseProvider
}

// OpenAIRequest represents an OpenAI Chat Completions request
type OpenAIRequest struct {
	Model            string              `json:"model"`
	Messages         []OpenAIMessage     `json:"messages"`
	MaxTokens        int                 `json:"max_tokens,omitempty"`
	Temperature      *float64            `json:"temperature,omitempty"`
	TopP             *float64            `json:"top_p,omitempty"`
	Stream           bool                `json:"stream"`
	Stop             []string            `json:"stop,omitempty"`
	PresencePenalty  *float64            `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64            `json:"frequency_penalty,omitempty"`
}

// OpenAIMessage represents a message in OpenAI format
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIResponse represents an OpenAI Chat Completions response
type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
}

// OpenAIChoice represents a choice in OpenAI response
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// OpenAIUsage represents token usage in OpenAI response
type OpenAIUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// SendRequest sends a request to OpenAI API
func (p *OpenAIProvider) SendRequest(ctx context.Context, ch *config.Channel, apiKey string, request []byte, stream bool) (*Response, error) {
	baseURLs := ch.GetAllBaseURLs()
	if len(baseURLs) == 0 {
		return nil, fmt.Errorf("no base URL configured")
	}

	url := baseURLs[0] + "/v1/chat/completions"

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + apiKey,
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
		var openaiResp OpenAIResponse
		if err := json.Unmarshal(body, &openaiResp); err == nil {
			response.InputTokens = openaiResp.Usage.PromptTokens
			response.OutputTokens = openaiResp.Usage.CompletionTokens
		}
	}

	return response, nil
}

// ConvertRequest converts Claude format to OpenAI format
func (p *OpenAIProvider) ConvertRequest(request []byte, model string) ([]byte, error) {
	var claudeReq ClaudeRequest
	if err := json.Unmarshal(request, &claudeReq); err != nil {
		return nil, fmt.Errorf("failed to parse Claude request: %w", err)
	}

	// Parse Claude messages
	var claudeMessages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(claudeReq.Messages, &claudeMessages); err != nil {
		return nil, fmt.Errorf("failed to parse messages: %w", err)
	}

	// Convert to OpenAI messages
	openaiMessages := make([]OpenAIMessage, 0, len(claudeMessages)+1)

	// Add system message if present
	if len(claudeReq.System) > 0 {
		var systemContent string
		if err := json.Unmarshal(claudeReq.System, &systemContent); err == nil {
			openaiMessages = append(openaiMessages, OpenAIMessage{
				Role:    "system",
				Content: systemContent,
			})
		}
	}

	// Convert messages
	for _, msg := range claudeMessages {
		content := extractTextContent(msg.Content)
		openaiMessages = append(openaiMessages, OpenAIMessage{
			Role:    msg.Role,
			Content: content,
		})
	}

	// Build OpenAI request
	openaiReq := OpenAIRequest{
		Model:       model,
		Messages:    openaiMessages,
		MaxTokens:   claudeReq.MaxTokens,
		Temperature: claudeReq.Temperature,
		TopP:        claudeReq.TopP,
		Stream:      claudeReq.Stream,
		Stop:        claudeReq.StopSequences,
	}

	return json.Marshal(openaiReq)
}

// ConvertResponse converts OpenAI response to Claude format
func (p *OpenAIProvider) ConvertResponse(response []byte) ([]byte, error) {
	var openaiResp OpenAIResponse
	if err := json.Unmarshal(response, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	// Extract content from first choice
	var content string
	var stopReason string
	if len(openaiResp.Choices) > 0 {
		content = openaiResp.Choices[0].Message.Content
		stopReason = convertOpenAIStopReason(openaiResp.Choices[0].FinishReason)
	}

	// Build Claude response
	claudeResp := ClaudeResponse{
		ID:         openaiResp.ID,
		Type:       "message",
		Role:       "assistant",
		Model:      openaiResp.Model,
		StopReason: stopReason,
		Content: []ContentBlock{
			{Type: "text", Text: content},
		},
		Usage: ClaudeUsage{
			InputTokens:  openaiResp.Usage.PromptTokens,
			OutputTokens: openaiResp.Usage.CompletionTokens,
		},
	}

	return json.Marshal(claudeResp)
}

// ParseStreamEvent parses an OpenAI streaming event
func (p *OpenAIProvider) ParseStreamEvent(event []byte) (*StreamEvent, error) {
	// OpenAI SSE format: "data: {...}\n\n"
	line := string(event)
	if !strings.HasPrefix(line, "data: ") {
		return nil, nil
	}

	data := strings.TrimPrefix(line, "data: ")
	if data == "[DONE]" {
		return &StreamEvent{Type: "done"}, nil
	}

	var chunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}

	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil, err
	}

	streamEvent := &StreamEvent{
		Type: "content_block_delta",
		Data: []byte(data),
	}

	if len(chunk.Choices) > 0 {
		streamEvent.Delta = chunk.Choices[0].Delta.Content
		if chunk.Choices[0].FinishReason != "" {
			streamEvent.StopReason = convertOpenAIStopReason(chunk.Choices[0].FinishReason)
		}
	}

	return streamEvent, nil
}

// extractTextContent extracts text from Claude content format
func extractTextContent(content json.RawMessage) string {
	// Try as string first
	var str string
	if err := json.Unmarshal(content, &str); err == nil {
		return str
	}

	// Try as array of content blocks
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(content, &blocks); err == nil {
		var result strings.Builder
		for _, block := range blocks {
			if block.Type == "text" {
				result.WriteString(block.Text)
			}
		}
		return result.String()
	}

	return string(content)
}

// convertOpenAIStopReason converts OpenAI finish reason to Claude stop reason
func convertOpenAIStopReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "content_filter":
		return "stop_sequence"
	default:
		return reason
	}
}
