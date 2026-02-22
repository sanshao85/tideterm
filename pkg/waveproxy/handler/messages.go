// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/sanshao85/tideterm/pkg/waveproxy/channel"
	"github.com/sanshao85/tideterm/pkg/waveproxy/config"
	"github.com/sanshao85/tideterm/pkg/waveproxy/history"
	"github.com/sanshao85/tideterm/pkg/waveproxy/metrics"
	"github.com/sanshao85/tideterm/pkg/waveproxy/scheduler"
)

// MessagesRequest represents a Claude Messages API request
type MessagesRequest struct {
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

type upstreamAttemptResult struct {
	ok           bool
	statusCode   int
	headers      http.Header
	body         []byte
	stream       io.ReadCloser
	apiKeyUsed   string
	inputTokens  int64
	outputTokens int64
	cacheRead    int64
	cacheCreate  int64
	errorMsg     string
	errorDetails string
}

// MessagesHandler handles the /v1/messages endpoint
func MessagesHandler(cfg *config.Config, sched *scheduler.Scheduler, metricsManager *metrics.Manager, historyManager *history.Manager) http.HandlerFunc {
	return AuthMiddleware(cfg, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			ErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, "failed to read request body")
			return
		}
		defer r.Body.Close()

		// Parse request
		var req MessagesRequest
		if err := json.Unmarshal(body, &req); err != nil {
			ErrorResponse(w, http.StatusBadRequest, "invalid JSON request")
			return
		}
		requestID := fmt.Sprintf("%x", time.Now().UnixNano())

		// Get user ID for affinity
		userID := extractClaudeCodeSessionID(req.Metadata)
		if userID == "" {
			userID = strings.TrimSpace(r.Header.Get("x-user-id"))
		}
		log.Printf("[WaveProxy-Messages] req=%s start stream=%v model=%s ua=%q accept=%q helper=%q beta=%q",
			requestID,
			req.Stream,
			req.Model,
			r.Header.Get("user-agent"),
			r.Header.Get("accept"),
			r.Header.Get("x-stainless-helper-method"),
			r.Header.Get("anthropic-beta"),
		)

		// Select channel
		excludeChannels := make(map[string]bool)
		maxRetries := 3
		var lastFailure *bufferedHTTPResponse

		for attempt := 0; attempt < maxRetries; attempt++ {
			ch, err := sched.SelectChannel(channel.ChannelTypeMessages, userID, excludeChannels)
			if err != nil {
				// If we've already attempted at least one upstream request, return its response
				// instead of overwriting it with a generic "no available channels" error.
				if lastFailure != nil {
					lastFailure.writeTo(w)
					return
				}
				if attempt == maxRetries-1 {
					ErrorResponse(w, http.StatusServiceUnavailable, "no available channels for messages endpoint")
					return
				}
				continue
			}

			startTime := time.Now()
			affinityKey, _ := sched.GetKeyAffinity(userID, ch.ID)

			// Apply model mapping
			model := req.Model
			if ch.ModelMapping != nil {
				if mapped, ok := ch.ModelMapping[model]; ok {
					model = mapped
				}
			}

			// Forward request to upstream
			result := proxyToUpstream(r, requestID, ch, body, model, req.Stream, historyManager, affinityKey)
			if result == nil {
				result = &upstreamAttemptResult{
					ok:           false,
					statusCode:   http.StatusBadGateway,
					headers:      http.Header{"Content-Type": []string{"application/json"}},
					body:         jsonErrorResponse(http.StatusBadGateway, "upstream request failed").body,
					errorMsg:     "upstream request failed",
					errorDetails: "upstream request failed",
				}
			}

			latencyMs := time.Since(startTime).Milliseconds()

			// Record metrics
			metricsManager.RecordRequest(
				ch.ID,
				result.ok,
				latencyMs,
				result.inputTokens,
				result.outputTokens,
				result.cacheRead,
				result.cacheCreate,
			)

			// Record to history
			if historyManager != nil {
				historyManager.RecordRequest(
					ch.ID,
					"messages",
					model,
					result.ok,
					latencyMs,
					result.inputTokens,
					result.outputTokens,
					result.errorMsg,
					result.errorDetails,
				)
			}

			if result.ok {
				sched.RecordSuccess(ch.ID)
				if userID != "" && result.apiKeyUsed != "" {
					sched.SetKeyAffinity(userID, ch.ID, result.apiKeyUsed, keyAffinityTTLForChannelType(channel.ChannelTypeMessages))
				}
				// Successful response: write to client once.
				if req.Stream {
					// IMPORTANT: Filter hop-by-hop headers from upstream (e.g. Connection/Transfer-Encoding)
					// to avoid confusing downstream clients and the net/http server.
					copyHeadersForDownstreamResponse(w.Header(), result.headers)
					if w.Header().Get("Content-Type") == "" {
						// Fall back to a reasonable default for Claude streaming responses.
						w.Header().Set("Content-Type", "text/event-stream")
					}
					w.WriteHeader(result.statusCode)
					if result.stream != nil {
						defer result.stream.Close()
						flusher, ok := w.(http.Flusher)
						if ok {
							// Flush headers immediately so the client can start processing.
							flusher.Flush()
						}
						var totalWritten int64
						buf := make([]byte, 4096)
						for {
							n, err := result.stream.Read(buf)
							if n > 0 {
								_, _ = w.Write(buf[:n])
								totalWritten += int64(n)
								if ok {
									flusher.Flush()
								}
							}
							if err == io.EOF {
								break
							}
							if err != nil {
								break
							}
						}
						if totalWritten == 0 {
							log.Printf("[WaveProxy-Messages] req=%s warn: upstream stream ended with zero bytes", requestID)
						}
					}
				} else {
					(&bufferedHTTPResponse{
						statusCode: result.statusCode,
						headers:    result.headers,
						body:       result.body,
					}).writeTo(w)
				}
				return
			}

			// Record failure and try next channel
			sched.RecordFailure(ch.ID, isRetryableHTTPStatus(result.statusCode))
			excludeChannels[ch.ID] = true
			lastFailure = &bufferedHTTPResponse{
				statusCode: result.statusCode,
				headers:    result.headers,
				body:       result.body,
			}
			log.Printf("[Messages-Failover] Channel %s failed, trying next", ch.Name)
		}

		if lastFailure != nil {
			lastFailure.writeTo(w)
			return
		}
		ErrorResponse(w, http.StatusBadGateway, "all channels failed")
	})
}

// CountTokensHandler handles the /v1/messages/count_tokens endpoint
func CountTokensHandler(cfg *config.Config) http.HandlerFunc {
	return AuthMiddleware(cfg, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			ErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, "failed to read request body")
			return
		}
		defer r.Body.Close()

		// Estimate tokens locally (rough approximation)
		tokenCount := len(body) / 4 // ~4 bytes per token
		log.Printf("[WaveProxy-CountTokens] ua=%q bytes=%d input_tokens=%d", r.Header.Get("user-agent"), len(body), tokenCount)

		response := map[string]interface{}{
			"input_tokens": tokenCount,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
}

// proxyToUpstream forwards the request to the upstream provider
func proxyToUpstream(
	r *http.Request,
	requestID string,
	ch *config.Channel,
	body []byte,
	model string,
	stream bool,
	historyManager *history.Manager,
	affinityKey string,
) *upstreamAttemptResult {
	// Get base URL
	baseURLs := ch.GetAllBaseURLs()
	if len(baseURLs) == 0 {
		return &upstreamAttemptResult{
			ok:           false,
			statusCode:   http.StatusBadGateway,
			headers:      http.Header{"Content-Type": []string{"application/json"}},
			body:         jsonErrorResponse(http.StatusBadGateway, "no base URL configured for channel").body,
			errorMsg:     "no base URL configured for channel",
			errorDetails: "no base URL configured for channel",
		}
	}

	baseURL := baseURLs[0]

	// Determine authentication keys
	//
	// If the channel has configured keys (even if all paused), do NOT fall back to
	// passthrough auth from the incoming request. This keeps "pause key" meaningful.
	// If the channel has no configured keys, we run in passthrough mode.
	hasConfiguredKeyEntries := len(ch.APIKeys) > 0
	enabledKeys := ch.EnabledAPIKeys()

	var passthroughApiKey, passthroughAuthKey string
	if !hasConfiguredKeyEntries {
		passthroughApiKey = r.Header.Get("x-api-key")
		authHeader := r.Header.Get("authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			passthroughAuthKey = strings.TrimPrefix(authHeader, "Bearer ")
		}
		log.Printf("[Messages-Auth] Passthrough mode: x-api-key=%v, auth=%v", passthroughApiKey != "", passthroughAuthKey != "")
		if passthroughApiKey == "" && passthroughAuthKey == "" {
			return &upstreamAttemptResult{
				ok:           false,
				statusCode:   http.StatusUnauthorized,
				headers:      http.Header{"Content-Type": []string{"application/json"}},
				body:         jsonErrorResponse(http.StatusUnauthorized, "no authentication provided").body,
				errorMsg:     "no authentication provided",
				errorDetails: "no authentication provided",
			}
		}
	} else {
		if len(enabledKeys) == 0 {
			return &upstreamAttemptResult{
				ok:           false,
				statusCode:   http.StatusUnauthorized,
				headers:      http.Header{"Content-Type": []string{"application/json"}},
				body:         jsonErrorResponse(http.StatusUnauthorized, "no enabled API keys configured for channel").body,
				errorMsg:     "no enabled API keys configured for channel",
				errorDetails: "no enabled API keys configured for channel",
			}
		}
		log.Printf("[Messages-Auth] Using channel configured API key(s): enabled=%d total=%d", len(enabledKeys), len(ch.APIKeys))
	}

	// Build upstream URL based on service type
	var upstreamURL string
	switch ch.ServiceType {
	case "claude":
		upstreamURL = baseURL + "/v1/messages"
	case "openai":
		upstreamURL = baseURL + "/v1/chat/completions"
	default:
		upstreamURL = baseURL + "/v1/messages"
	}

	// Modify request body with mapped model
	var reqBody map[string]interface{}
	_ = json.Unmarshal(body, &reqBody)
	toolsCount := -1
	if toolsVal, ok := reqBody["tools"]; ok {
		if toolsArr, ok := toolsVal.([]interface{}); ok {
			toolsCount = len(toolsArr)
		} else {
			toolsCount = 0
		}
	}
	msgCount := -1
	if msgsVal, ok := reqBody["messages"]; ok {
		if msgsArr, ok := msgsVal.([]interface{}); ok {
			msgCount = len(msgsArr)
		}
	}
	reqBody["model"] = model
	modifiedBody, _ := json.Marshal(reqBody)

	// Set auth headers based on AuthType
	authType := ch.AuthType
	if authType == "" {
		// Default based on service type
		switch ch.ServiceType {
		case "openai":
			authType = config.AuthTypeBearer
		default:
			authType = config.AuthTypeAPIKey
		}
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	keyAttempts := []string{""}
	if hasConfiguredKeyEntries {
		keyAttempts = applyAffinityKeyOrder(enabledKeys, affinityKey)
	}

	for keyIndex, key := range keyAttempts {
		// Create upstream request
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		upstreamStart := time.Now()

		upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(modifiedBody))
		if err != nil {
			cancel()
			return &upstreamAttemptResult{
				ok:           false,
				statusCode:   http.StatusInternalServerError,
				headers:      http.Header{"Content-Type": []string{"application/json"}},
				body:         jsonErrorResponse(http.StatusInternalServerError, "failed to create upstream request").body,
				errorMsg:     "failed to create upstream request",
				errorDetails: "failed to create upstream request",
			}
		}

		// Start with a safe passthrough of client headers (e.g. User-Agent) for compatibility
		// with upstreams that gate access by client identity.
		copyHeadersForUpstreamRequest(upstreamReq.Header, r.Header)

		// Set headers
		upstreamReq.Header.Set("Content-Type", "application/json")
		if upstreamReq.Header.Get("anthropic-version") == "" {
			upstreamReq.Header.Set("anthropic-version", "2023-06-01")
		}

		// Prevent mixing client-provided tokens with configured keys.
		if hasConfiguredKeyEntries {
			upstreamReq.Header.Del("X-Api-Key")
			upstreamReq.Header.Del("Authorization")
		}
		if stream {
			// Some upstreams (notably certain third-party proxies) only enable SSE when the
			// client explicitly advertises it. Claude Code relies on streaming responses,
			// so force an SSE-friendly Accept header when stream=true.
			if !strings.Contains(strings.ToLower(upstreamReq.Header.Get("accept")), "text/event-stream") {
				upstreamReq.Header.Set("Accept", "text/event-stream")
			}
		}

		// Determine per-attempt keys
		keyForApiKey := passthroughApiKey
		keyForAuth := passthroughAuthKey
		if hasConfiguredKeyEntries {
			keyForApiKey = key
			keyForAuth = key
		}
		apiKeyUsed := keyForAuth
		if apiKeyUsed == "" {
			apiKeyUsed = keyForApiKey
		}

		// Set auth headers based on AuthType
		switch authType {
		case config.AuthTypeBearer:
			authKey := keyForAuth
			if authKey == "" {
				authKey = keyForApiKey
			}
			upstreamReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authKey))
			log.Printf("[Messages-Auth] Sending Bearer token only")
		case config.AuthTypeBoth:
			apiKey := keyForApiKey
			if apiKey == "" {
				apiKey = keyForAuth
			}
			authKey := keyForAuth
			if authKey == "" {
				authKey = keyForApiKey
			}
			upstreamReq.Header.Set("x-api-key", apiKey)
			upstreamReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authKey))
			log.Printf("[Messages-Auth] Sending both x-api-key and Bearer token")
		default: // AuthTypeAPIKey
			apiKey := keyForApiKey
			if apiKey == "" {
				apiKey = keyForAuth
			}
			upstreamReq.Header.Set("x-api-key", apiKey)
			log.Printf("[Messages-Auth] Sending x-api-key only")
		}

		resp, err := client.Do(upstreamReq)
		if err != nil {
			cancel()
			log.Printf("[WaveProxy-Messages] req=%s upstream error: channel=%s url=%s err=%v", requestID, ch.ID, redactURLForLogs(upstreamURL), err)
			log.Printf("[Messages-Error] Upstream request failed: %v", err)
			return &upstreamAttemptResult{
				ok:           false,
				statusCode:   http.StatusBadGateway,
				headers:      http.Header{"Content-Type": []string{"application/json"}},
				body:         jsonErrorResponse(http.StatusBadGateway, "upstream request failed").body,
				errorMsg:     "upstream request failed: " + redactSecretsForLogs(err.Error()),
				errorDetails: "upstream request failed: " + redactSecretsForLogs(err.Error()),
			}
		}

		log.Printf("[WaveProxy-Messages] req=%s upstream response: channel=%s url=%s status=%d ct=%q ce=%q te=%q cl=%q header_ms=%d tools=%d messages=%d",
			requestID,
			ch.ID,
			redactURLForLogs(upstreamURL),
			resp.StatusCode,
			resp.Header.Get("Content-Type"),
			resp.Header.Get("Content-Encoding"),
			resp.Header.Get("Transfer-Encoding"),
			resp.Header.Get("Content-Length"),
			time.Since(upstreamStart).Milliseconds(),
			toolsCount,
			msgCount,
		)

		// Check status
		if resp.StatusCode >= 400 {
			respBody, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			cancel()
			normalizedHeaders, normalizedBody := normalizeUpstreamErrorResponse(resp.StatusCode, resp.Header.Clone(), respBody)
			errMsg := extractErrorMessage(normalizedBody)
			if errMsg == "" {
				errMsg = "upstream returned error"
			}
			errorDetails := bodySnippetForLogs(respBody, 8192)
			log.Printf("[Messages-Error] Upstream returned %d: %s", resp.StatusCode, bodySnippetForLogs(respBody, 2048))
			if hasConfiguredKeyEntries && keyIndex < len(keyAttempts)-1 && isRetryableWithAnotherAPIKey(resp.StatusCode) {
				if historyManager != nil {
					historyManager.RecordRequest(
						ch.ID,
						"messages",
						model,
						false,
						time.Since(upstreamStart).Milliseconds(),
						0,
						0,
						fmt.Sprintf("API key %d/%d failed: HTTP %d: %s", keyIndex+1, len(keyAttempts), resp.StatusCode, errMsg),
						errorDetails,
					)
				}
				log.Printf("[Messages-Auth] Upstream returned %d; trying next API key (%d/%d)", resp.StatusCode, keyIndex+2, len(keyAttempts))
				continue
			}
			return &upstreamAttemptResult{
				ok:           false,
				statusCode:   resp.StatusCode,
				headers:      normalizedHeaders,
				body:         normalizedBody,
				errorMsg:     fmt.Sprintf("HTTP %d: %s", resp.StatusCode, errMsg),
				errorDetails: errorDetails,
			}
		}

		result := &upstreamAttemptResult{
			ok:         true,
			statusCode: resp.StatusCode,
			headers:    resp.Header.Clone(),
			apiKeyUsed: apiKeyUsed,
		}

		if stream {
			// Caller streams response; ownership of resp.Body transferred.
			streamBody := cancelOnCloseReadCloser{
				ReadCloser: resp.Body,
				cancel:     cancel,
			}
			// Guard against a 200 response that ends immediately with zero bytes (breaks Claude Code's JSON parsing).
			firstByte := make([]byte, 1)
			n, err := streamBody.Read(firstByte)
			if n == 0 && err != nil {
				_ = streamBody.Close()
				log.Printf("[WaveProxy-Messages] req=%s upstream stream ended before first byte: err=%v", requestID, err)
				return &upstreamAttemptResult{
					ok:           false,
					statusCode:   http.StatusBadGateway,
					headers:      http.Header{"Content-Type": []string{"application/json"}},
					body:         jsonErrorResponse(http.StatusBadGateway, "upstream stream ended before first byte").body,
					errorMsg:     "upstream stream ended before first byte",
					errorDetails: "upstream stream ended before first byte",
				}
			}
			if n > 0 {
				result.stream = &prefixedReadCloser{
					prefix:     firstByte[:n],
					ReadCloser: streamBody,
				}
			} else {
				result.stream = streamBody
			}
			return result
		}

		respBody, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		if err == nil {
			result.body = respBody

			// Parse response for token counts
			var respData struct {
				Usage struct {
					InputTokens       int64 `json:"input_tokens"`
					OutputTokens      int64 `json:"output_tokens"`
					CacheReadTokens   int64 `json:"cache_read_input_tokens"`
					CacheCreateTokens int64 `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			}
			if json.Unmarshal(respBody, &respData) == nil {
				result.inputTokens = respData.Usage.InputTokens
				result.outputTokens = respData.Usage.OutputTokens
				result.cacheRead = respData.Usage.CacheReadTokens
				result.cacheCreate = respData.Usage.CacheCreateTokens
			}
			return result
		}

		return &upstreamAttemptResult{
			ok:           false,
			statusCode:   http.StatusBadGateway,
			headers:      http.Header{"Content-Type": []string{"application/json"}},
			body:         jsonErrorResponse(http.StatusBadGateway, "failed to read upstream response").body,
			errorMsg:     "failed to read upstream response",
			errorDetails: "failed to read upstream response",
		}
	}

	return &upstreamAttemptResult{
		ok:           false,
		statusCode:   http.StatusBadGateway,
		headers:      http.Header{"Content-Type": []string{"application/json"}},
		body:         jsonErrorResponse(http.StatusBadGateway, "upstream request failed").body,
		errorMsg:     "upstream request failed",
		errorDetails: "upstream request failed",
	}
}

type cancelOnCloseReadCloser struct {
	io.ReadCloser
	cancel func()
}

func (c cancelOnCloseReadCloser) Close() error {
	err := c.ReadCloser.Close()
	if c.cancel != nil {
		c.cancel()
	}
	return err
}

type prefixedReadCloser struct {
	prefix []byte
	io.ReadCloser
}

func (p *prefixedReadCloser) Read(buf []byte) (int, error) {
	if len(p.prefix) > 0 {
		n := copy(buf, p.prefix)
		p.prefix = p.prefix[n:]
		return n, nil
	}
	return p.ReadCloser.Read(buf)
}
