// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package converter

import (
	"encoding/json"
	"fmt"
)

// OpenAIConverter converts between OpenAI and Claude formats
type OpenAIConverter struct{}

// OpenAI types
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIRequest struct {
	Model            string          `json:"model"`
	Messages         []OpenAIMessage `json:"messages"`
	MaxTokens        int             `json:"max_tokens,omitempty"`
	Temperature      *float64        `json:"temperature,omitempty"`
	TopP             *float64        `json:"top_p,omitempty"`
	Stream           bool            `json:"stream"`
	Stop             []string        `json:"stop,omitempty"`
	PresencePenalty  *float64        `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64        `json:"frequency_penalty,omitempty"`
}

type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
}

type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type OpenAIUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

func (c *OpenAIConverter) Name() string {
	return "openai"
}

// ToClaude converts OpenAI request to Claude format
func (c *OpenAIConverter) ToClaude(request []byte) ([]byte, error) {
	var openaiReq OpenAIRequest
	if err := json.Unmarshal(request, &openaiReq); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI request: %w", err)
	}

	// Convert messages
	claudeMessages := make([]ClaudeMessage, 0)
	var systemContent string

	for _, msg := range openaiReq.Messages {
		if msg.Role == "system" {
			systemContent = msg.Content
			continue
		}

		claudeMessages = append(claudeMessages, ClaudeMessage{
			Role:    msg.Role,
			Content: BuildTextContent(msg.Content),
		})
	}

	claudeReq := ClaudeRequest{
		Model:         openaiReq.Model,
		MaxTokens:     openaiReq.MaxTokens,
		Messages:      claudeMessages,
		Stream:        openaiReq.Stream,
		Temperature:   openaiReq.Temperature,
		TopP:          openaiReq.TopP,
		StopSequences: openaiReq.Stop,
	}

	if systemContent != "" {
		claudeReq.System = systemContent
	}

	return json.Marshal(claudeReq)
}

// FromClaude converts Claude request to OpenAI format
func (c *OpenAIConverter) FromClaude(request []byte) ([]byte, error) {
	claudeReq, err := ParseClaudeRequest(request)
	if err != nil {
		return nil, err
	}

	openaiMessages := make([]OpenAIMessage, 0)

	// Add system message if present
	if claudeReq.System != nil {
		var systemStr string
		switch v := claudeReq.System.(type) {
		case string:
			systemStr = v
		default:
			data, _ := json.Marshal(v)
			systemStr = string(data)
		}
		openaiMessages = append(openaiMessages, OpenAIMessage{
			Role:    "system",
			Content: systemStr,
		})
	}

	// Convert messages
	for _, msg := range claudeReq.Messages {
		content := ExtractTextFromContent(msg.Content)
		openaiMessages = append(openaiMessages, OpenAIMessage{
			Role:    msg.Role,
			Content: content,
		})
	}

	openaiReq := OpenAIRequest{
		Model:       claudeReq.Model,
		Messages:    openaiMessages,
		MaxTokens:   claudeReq.MaxTokens,
		Temperature: claudeReq.Temperature,
		TopP:        claudeReq.TopP,
		Stream:      claudeReq.Stream,
		Stop:        claudeReq.StopSequences,
	}

	return json.Marshal(openaiReq)
}

// ResponseToClaude converts OpenAI response to Claude format
func (c *OpenAIConverter) ResponseToClaude(response []byte) ([]byte, error) {
	var openaiResp OpenAIResponse
	if err := json.Unmarshal(response, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	var content string
	var stopReason string

	if len(openaiResp.Choices) > 0 {
		content = openaiResp.Choices[0].Message.Content
		stopReason = c.convertStopReason(openaiResp.Choices[0].FinishReason)
	}

	claudeResp := ClaudeResponse{
		ID:         openaiResp.ID,
		Type:       "message",
		Role:       "assistant",
		Model:      openaiResp.Model,
		StopReason: stopReason,
		Content: []ClaudeContentBlock{
			{Type: "text", Text: content},
		},
		Usage: ClaudeUsage{
			InputTokens:  openaiResp.Usage.PromptTokens,
			OutputTokens: openaiResp.Usage.CompletionTokens,
		},
	}

	return json.Marshal(claudeResp)
}

// ResponseFromClaude converts Claude response to OpenAI format
func (c *OpenAIConverter) ResponseFromClaude(response []byte) ([]byte, error) {
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

	openaiResp := OpenAIResponse{
		ID:      claudeResp.ID,
		Object:  "chat.completion",
		Model:   claudeResp.Model,
		Choices: []OpenAIChoice{
			{
				Index: 0,
				Message: OpenAIMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: c.convertClaudeStopReason(claudeResp.StopReason),
			},
		},
		Usage: OpenAIUsage{
			PromptTokens:     claudeResp.Usage.InputTokens,
			CompletionTokens: claudeResp.Usage.OutputTokens,
			TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		},
	}

	return json.Marshal(openaiResp)
}

func (c *OpenAIConverter) convertStopReason(reason string) string {
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

func (c *OpenAIConverter) convertClaudeStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	default:
		return reason
	}
}
