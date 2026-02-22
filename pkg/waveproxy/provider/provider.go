// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Package provider provides upstream AI service adapters.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sanshao85/tideterm/pkg/waveproxy/config"
)

// Provider interface for all upstream AI services
type Provider interface {
	// Name returns the provider name
	Name() string

	// SendRequest sends a request to the upstream service
	SendRequest(ctx context.Context, ch *config.Channel, apiKey string, request []byte, stream bool) (*Response, error)

	// ConvertRequest converts a Claude-format request to provider format
	ConvertRequest(request []byte, model string) ([]byte, error)

	// ConvertResponse converts provider response to Claude format
	ConvertResponse(response []byte) ([]byte, error)

	// ParseStreamEvent parses a streaming event
	ParseStreamEvent(event []byte) (*StreamEvent, error)
}

// Response represents an upstream response
type Response struct {
	StatusCode   int
	Headers      http.Header
	Body         []byte
	Stream       io.ReadCloser
	InputTokens  int64
	OutputTokens int64
	CacheRead    int64
	CacheCreate  int64
}

// StreamEvent represents a streaming event
type StreamEvent struct {
	Type         string
	Data         json.RawMessage
	Delta        string
	InputTokens  int64
	OutputTokens int64
	StopReason   string
}

// GetProvider returns a provider instance by service type
func GetProvider(serviceType string) Provider {
	switch serviceType {
	case "claude":
		return &ClaudeProvider{}
	case "openai":
		return &OpenAIProvider{}
	case "gemini":
		return &GeminiProvider{}
	default:
		return &ClaudeProvider{} // Default to Claude
	}
}

// BaseProvider provides common functionality
type BaseProvider struct {
	client *http.Client
}

// NewBaseProvider creates a base provider with default HTTP client
func NewBaseProvider() BaseProvider {
	return BaseProvider{
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// DoRequest performs an HTTP request
func (p *BaseProvider) DoRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return p.client.Do(req)
}

// ReadResponse reads the full response body
func (p *BaseProvider) ReadResponse(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
