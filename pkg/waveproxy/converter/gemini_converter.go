// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package converter

import (
	"encoding/json"
	"fmt"
)

// GeminiConverter converts between Gemini and Claude formats
type GeminiConverter struct{}

// Gemini types
type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text,omitempty"`
}

type GeminiRequest struct {
	Contents          []GeminiContent         `json:"contents"`
	SystemInstruction *GeminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  *GeminiGenerationConfig `json:"generationConfig,omitempty"`
}

type GeminiGenerationConfig struct {
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	TopK            *int     `json:"topK,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type GeminiResponse struct {
	Candidates    []GeminiCandidate   `json:"candidates"`
	UsageMetadata GeminiUsageMetadata `json:"usageMetadata"`
	ModelVersion  string              `json:"modelVersion"`
}

type GeminiCandidate struct {
	Content      GeminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
	Index        int           `json:"index"`
}

type GeminiUsageMetadata struct {
	PromptTokenCount     int64 `json:"promptTokenCount"`
	CandidatesTokenCount int64 `json:"candidatesTokenCount"`
	TotalTokenCount      int64 `json:"totalTokenCount"`
}

func (c *GeminiConverter) Name() string {
	return "gemini"
}

// ToClaude converts Gemini request to Claude format
func (c *GeminiConverter) ToClaude(request []byte) ([]byte, error) {
	var geminiReq GeminiRequest
	if err := json.Unmarshal(request, &geminiReq); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini request: %w", err)
	}

	// Convert contents to messages
	claudeMessages := make([]ClaudeMessage, 0)
	for _, content := range geminiReq.Contents {
		role := content.Role
		if role == "model" {
			role = "assistant"
		}

		var text string
		for _, part := range content.Parts {
			text += part.Text
		}

		claudeMessages = append(claudeMessages, ClaudeMessage{
			Role:    role,
			Content: BuildTextContent(text),
		})
	}

	claudeReq := ClaudeRequest{
		Model:    "claude-3-sonnet-20240229", // Default model
		Messages: claudeMessages,
	}

	// Convert generation config
	if geminiReq.GenerationConfig != nil {
		claudeReq.MaxTokens = geminiReq.GenerationConfig.MaxOutputTokens
		claudeReq.Temperature = geminiReq.GenerationConfig.Temperature
		claudeReq.TopP = geminiReq.GenerationConfig.TopP
		claudeReq.TopK = geminiReq.GenerationConfig.TopK
		claudeReq.StopSequences = geminiReq.GenerationConfig.StopSequences
	}

	// Convert system instruction
	if geminiReq.SystemInstruction != nil && len(geminiReq.SystemInstruction.Parts) > 0 {
		claudeReq.System = geminiReq.SystemInstruction.Parts[0].Text
	}

	return json.Marshal(claudeReq)
}

// FromClaude converts Claude request to Gemini format
func (c *GeminiConverter) FromClaude(request []byte) ([]byte, error) {
	claudeReq, err := ParseClaudeRequest(request)
	if err != nil {
		return nil, err
	}

	// Convert messages to contents
	geminiContents := make([]GeminiContent, 0)
	for _, msg := range claudeReq.Messages {
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}

		content := ExtractTextFromContent(msg.Content)
		geminiContents = append(geminiContents, GeminiContent{
			Role: role,
			Parts: []GeminiPart{
				{Text: content},
			},
		})
	}

	geminiReq := GeminiRequest{
		Contents: geminiContents,
		GenerationConfig: &GeminiGenerationConfig{
			MaxOutputTokens: claudeReq.MaxTokens,
			Temperature:     claudeReq.Temperature,
			TopP:            claudeReq.TopP,
			TopK:            claudeReq.TopK,
			StopSequences:   claudeReq.StopSequences,
		},
	}

	// Add system instruction
	if claudeReq.System != nil {
		var systemStr string
		switch v := claudeReq.System.(type) {
		case string:
			systemStr = v
		default:
			data, _ := json.Marshal(v)
			systemStr = string(data)
		}
		geminiReq.SystemInstruction = &GeminiContent{
			Parts: []GeminiPart{
				{Text: systemStr},
			},
		}
	}

	return json.Marshal(geminiReq)
}

// ResponseToClaude converts Gemini response to Claude format
func (c *GeminiConverter) ResponseToClaude(response []byte) ([]byte, error) {
	var geminiResp GeminiResponse
	if err := json.Unmarshal(response, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	var content string
	var stopReason string

	if len(geminiResp.Candidates) > 0 {
		candidate := geminiResp.Candidates[0]
		for _, part := range candidate.Content.Parts {
			content += part.Text
		}
		stopReason = c.convertStopReason(candidate.FinishReason)
	}

	claudeResp := ClaudeResponse{
		ID:         fmt.Sprintf("msg_%d", geminiResp.UsageMetadata.TotalTokenCount),
		Type:       "message",
		Role:       "assistant",
		Model:      geminiResp.ModelVersion,
		StopReason: stopReason,
		Content: []ClaudeContentBlock{
			{Type: "text", Text: content},
		},
		Usage: ClaudeUsage{
			InputTokens:  geminiResp.UsageMetadata.PromptTokenCount,
			OutputTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
		},
	}

	return json.Marshal(claudeResp)
}

// ResponseFromClaude converts Claude response to Gemini format
func (c *GeminiConverter) ResponseFromClaude(response []byte) ([]byte, error) {
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

	geminiResp := GeminiResponse{
		Candidates: []GeminiCandidate{
			{
				Content: GeminiContent{
					Role: "model",
					Parts: []GeminiPart{
						{Text: content},
					},
				},
				FinishReason: c.convertClaudeStopReason(claudeResp.StopReason),
				Index:        0,
			},
		},
		UsageMetadata: GeminiUsageMetadata{
			PromptTokenCount:     claudeResp.Usage.InputTokens,
			CandidatesTokenCount: claudeResp.Usage.OutputTokens,
			TotalTokenCount:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		},
		ModelVersion: claudeResp.Model,
	}

	return json.Marshal(geminiResp)
}

func (c *GeminiConverter) convertStopReason(reason string) string {
	switch reason {
	case "STOP":
		return "end_turn"
	case "MAX_TOKENS":
		return "max_tokens"
	case "SAFETY":
		return "stop_sequence"
	case "RECITATION":
		return "stop_sequence"
	default:
		return reason
	}
}

func (c *GeminiConverter) convertClaudeStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "STOP"
	case "max_tokens":
		return "MAX_TOKENS"
	case "stop_sequence":
		return "STOP"
	default:
		return reason
	}
}
