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

// GeminiProvider handles Google Gemini API requests
type GeminiProvider struct {
	BaseProvider
}

// GeminiRequest represents a Gemini generateContent request
type GeminiRequest struct {
	Contents         []GeminiContent        `json:"contents"`
	SystemInstruction *GeminiContent        `json:"systemInstruction,omitempty"`
	GenerationConfig *GeminiGenerationConfig `json:"generationConfig,omitempty"`
}

// GeminiContent represents content in Gemini format
type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart represents a part of content
type GeminiPart struct {
	Text string `json:"text,omitempty"`
}

// GeminiGenerationConfig represents generation configuration
type GeminiGenerationConfig struct {
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	TopK            *int     `json:"topK,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

// GeminiResponse represents a Gemini API response
type GeminiResponse struct {
	Candidates     []GeminiCandidate   `json:"candidates"`
	UsageMetadata  GeminiUsageMetadata `json:"usageMetadata"`
	ModelVersion   string              `json:"modelVersion"`
}

// GeminiCandidate represents a response candidate
type GeminiCandidate struct {
	Content       GeminiContent `json:"content"`
	FinishReason  string        `json:"finishReason"`
	Index         int           `json:"index"`
	SafetyRatings []interface{} `json:"safetyRatings"`
}

// GeminiUsageMetadata represents token usage
type GeminiUsageMetadata struct {
	PromptTokenCount     int64 `json:"promptTokenCount"`
	CandidatesTokenCount int64 `json:"candidatesTokenCount"`
	TotalTokenCount      int64 `json:"totalTokenCount"`
}

// Name returns the provider name
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// SendRequest sends a request to Gemini API
func (p *GeminiProvider) SendRequest(ctx context.Context, ch *config.Channel, apiKey string, request []byte, stream bool) (*Response, error) {
	baseURLs := ch.GetAllBaseURLs()
	if len(baseURLs) == 0 {
		return nil, fmt.Errorf("no base URL configured")
	}

	// Gemini uses query parameter for API key
	model := "gemini-pro" // Default model
	action := "generateContent"
	if stream {
		action = "streamGenerateContent"
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:%s?key=%s", baseURLs[0], model, action, apiKey)

	headers := map[string]string{
		"Content-Type": "application/json",
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
		var geminiResp GeminiResponse
		if err := json.Unmarshal(body, &geminiResp); err == nil {
			response.InputTokens = geminiResp.UsageMetadata.PromptTokenCount
			response.OutputTokens = geminiResp.UsageMetadata.CandidatesTokenCount
		}
	}

	return response, nil
}

// ConvertRequest converts Claude format to Gemini format
func (p *GeminiProvider) ConvertRequest(request []byte, model string) ([]byte, error) {
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

	// Convert to Gemini contents
	geminiContents := make([]GeminiContent, 0, len(claudeMessages))
	for _, msg := range claudeMessages {
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}

		content := extractTextContent(msg.Content)
		geminiContents = append(geminiContents, GeminiContent{
			Role: role,
			Parts: []GeminiPart{
				{Text: content},
			},
		})
	}

	// Build Gemini request
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

	// Add system instruction if present
	if len(claudeReq.System) > 0 {
		var systemContent string
		if err := json.Unmarshal(claudeReq.System, &systemContent); err == nil {
			geminiReq.SystemInstruction = &GeminiContent{
				Parts: []GeminiPart{
					{Text: systemContent},
				},
			}
		}
	}

	return json.Marshal(geminiReq)
}

// ConvertResponse converts Gemini response to Claude format
func (p *GeminiProvider) ConvertResponse(response []byte) ([]byte, error) {
	var geminiResp GeminiResponse
	if err := json.Unmarshal(response, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	// Extract content from first candidate
	var content string
	var stopReason string
	if len(geminiResp.Candidates) > 0 {
		candidate := geminiResp.Candidates[0]
		if len(candidate.Content.Parts) > 0 {
			content = candidate.Content.Parts[0].Text
		}
		stopReason = convertGeminiStopReason(candidate.FinishReason)
	}

	// Build Claude response
	claudeResp := ClaudeResponse{
		ID:         fmt.Sprintf("msg_%d", geminiResp.UsageMetadata.TotalTokenCount),
		Type:       "message",
		Role:       "assistant",
		Model:      geminiResp.ModelVersion,
		StopReason: stopReason,
		Content: []ContentBlock{
			{Type: "text", Text: content},
		},
		Usage: ClaudeUsage{
			InputTokens:  geminiResp.UsageMetadata.PromptTokenCount,
			OutputTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
		},
	}

	return json.Marshal(claudeResp)
}

// ParseStreamEvent parses a Gemini streaming event
func (p *GeminiProvider) ParseStreamEvent(event []byte) (*StreamEvent, error) {
	// Gemini streams as JSON objects
	line := strings.TrimSpace(string(event))
	if line == "" {
		return nil, nil
	}

	// Remove "data: " prefix if present
	if strings.HasPrefix(line, "data: ") {
		line = strings.TrimPrefix(line, "data: ")
	}

	var chunk struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata GeminiUsageMetadata `json:"usageMetadata"`
	}

	if err := json.Unmarshal([]byte(line), &chunk); err != nil {
		return nil, nil // Skip unparseable chunks
	}

	streamEvent := &StreamEvent{
		Type: "content_block_delta",
		Data: []byte(line),
	}

	if len(chunk.Candidates) > 0 {
		candidate := chunk.Candidates[0]
		if len(candidate.Content.Parts) > 0 {
			streamEvent.Delta = candidate.Content.Parts[0].Text
		}
		if candidate.FinishReason != "" {
			streamEvent.StopReason = convertGeminiStopReason(candidate.FinishReason)
		}
	}

	streamEvent.InputTokens = chunk.UsageMetadata.PromptTokenCount
	streamEvent.OutputTokens = chunk.UsageMetadata.CandidatesTokenCount

	return streamEvent, nil
}

// convertGeminiStopReason converts Gemini finish reason to Claude stop reason
func convertGeminiStopReason(reason string) string {
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
