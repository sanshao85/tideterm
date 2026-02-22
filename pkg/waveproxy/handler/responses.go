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
	"github.com/sanshao85/tideterm/pkg/waveproxy/session"
)

// ResponsesRequest represents a Responses API request
type ResponsesRequest struct {
	Model              string          `json:"model"`
	MaxOutputTokens    int             `json:"max_output_tokens,omitempty"`
	Input              json.RawMessage `json:"input"`
	Instructions       string          `json:"instructions,omitempty"`
	PreviousResponseID string          `json:"previous_response_id,omitempty"`
	PromptCacheKey     string          `json:"prompt_cache_key,omitempty"`
	Stream             bool            `json:"stream"`
	Temperature        *float64        `json:"temperature,omitempty"`
}

type responsesAttemptResult struct {
	ok           bool
	statusCode   int
	headers      http.Header
	body         []byte
	stream       io.ReadCloser
	responseID   string
	apiKeyUsed   string
	inputTokens  int64
	outputTokens int64
	cacheRead    int64
	cacheCreate  int64
	errorMsg     string
	errorDetails string
}

type geminiAttemptResult struct {
	ok           bool
	statusCode   int
	headers      http.Header
	body         []byte
	stream       io.ReadCloser
	apiKeyUsed   string
	inputTokens  int64
	outputTokens int64
	cacheRead    int64
	errorMsg     string
	errorDetails string
}

// ResponsesHandler handles the /v1/responses endpoint
func ResponsesHandler(cfg *config.Config, sched *scheduler.Scheduler, metricsManager *metrics.Manager, sessionMgr *session.Manager, historyManager *history.Manager) http.HandlerFunc {
	return AuthMiddleware(cfg, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			ErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		requestID := fmt.Sprintf("%x", time.Now().UnixNano())

		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, "failed to read request body")
			return
		}
		defer r.Body.Close()

		// Parse request
		var req ResponsesRequest
		if err := json.Unmarshal(body, &req); err != nil {
			ErrorResponse(w, http.StatusBadRequest, "invalid JSON request")
			return
		}

		// Get user ID for affinity
		userID := ""
		if strings.TrimSpace(req.PromptCacheKey) != "" {
			userID = "codex_" + strings.TrimSpace(req.PromptCacheKey)
		}
		if userID == "" && req.PreviousResponseID != "" {
			userID = req.PreviousResponseID
		}
		if userID == "" {
			userID = strings.TrimSpace(r.Header.Get("x-user-id"))
		}

		// Select channel
		excludeChannels := make(map[string]bool)
		maxRetries := 3
		var lastFailure *bufferedHTTPResponse

		for attempt := 0; attempt < maxRetries; attempt++ {
			ch, err := sched.SelectChannel(channel.ChannelTypeResponses, userID, excludeChannels)
			if err != nil {
				if lastFailure != nil {
					lastFailure.writeTo(w)
					return
				}
				if attempt == maxRetries-1 {
					ErrorResponse(w, http.StatusServiceUnavailable, "no available channels for responses endpoint")
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

			// Forward request
			var result *responsesAttemptResult
			switch strings.TrimSpace(ch.ServiceType) {
			case "openai":
				result = proxyOpenAIResponsesRequest(r, requestID, ch, body, model, req.Stream, historyManager, affinityKey)
			default:
				// Claude-style upstream: convert OpenAI Responses API -> Claude Messages API.
				sess, _ := sessionMgr.GetOrCreateSession(req.PreviousResponseID)
				messages := buildMessagesFromSession(sess, req)
				result = proxyClaudeResponsesRequest(r, ch, model, messages, req, sess, sessionMgr, historyManager, affinityKey)
			}
			if result == nil {
				result = &responsesAttemptResult{
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
					"responses",
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
					sched.SetKeyAffinity(userID, ch.ID, result.apiKeyUsed, keyAffinityTTLForChannelType(channel.ChannelTypeResponses))
				}
				log.Printf("[Responses-Success] req=%s channel=%s stream=%v response_id=%s", requestID, ch.ID, req.Stream, result.responseID)

				if req.Stream && result.stream != nil {
					copyHeadersForDownstreamResponse(w.Header(), result.headers)
					if w.Header().Get("Content-Type") == "" {
						w.Header().Set("Content-Type", "text/event-stream")
					}
					w.WriteHeader(result.statusCode)
					defer result.stream.Close()

					flusher, ok := w.(http.Flusher)
					if ok {
						flusher.Flush()
					}
					buf := make([]byte, 4096)
					for {
						n, err := result.stream.Read(buf)
						if n > 0 {
							_, _ = w.Write(buf[:n])
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
					return
				}

				(&bufferedHTTPResponse{
					statusCode: result.statusCode,
					headers:    result.headers,
					body:       result.body,
				}).writeTo(w)
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
			log.Printf("[Responses-Failover] Channel %s failed, trying next", ch.Name)
		}

		if lastFailure != nil {
			lastFailure.writeTo(w)
			return
		}
		ErrorResponse(w, http.StatusBadGateway, "all channels failed")
	})
}

// ResponsesCompactHandler handles the /v1/responses/compact endpoint
func ResponsesCompactHandler(cfg *config.Config, sched *scheduler.Scheduler, metricsManager *metrics.Manager, sessionMgr *session.Manager, historyManager *history.Manager) http.HandlerFunc {
	// Same as ResponsesHandler but with compact output format
	return ResponsesHandler(cfg, sched, metricsManager, sessionMgr, historyManager)
}

// buildMessagesFromSession builds a messages array from session history
func buildMessagesFromSession(sess *session.Session, req ResponsesRequest) []map[string]interface{} {
	var messages []map[string]interface{}

	// Add history from session
	for _, msg := range sess.Messages {
		messages = append(messages, map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	// Parse and add current input
	var input interface{}
	if err := json.Unmarshal(req.Input, &input); err == nil {
		switch v := input.(type) {
		case string:
			messages = append(messages, map[string]interface{}{
				"role":    "user",
				"content": v,
			})
		case []interface{}:
			// Input is array of messages
			for _, item := range v {
				if msg, ok := item.(map[string]interface{}); ok {
					messages = append(messages, msg)
				}
			}
		}
	}

	return messages
}

// proxyClaudeResponsesRequest forwards the request to a Claude-style upstream (Messages API) and handles local session management.
func proxyClaudeResponsesRequest(
	r *http.Request,
	ch *config.Channel,
	model string,
	messages []map[string]interface{},
	req ResponsesRequest,
	sess *session.Session,
	sessionMgr *session.Manager,
	historyManager *history.Manager,
	affinityKey string,
) *responsesAttemptResult {
	// Get base URL
	baseURLs := ch.GetAllBaseURLs()
	if len(baseURLs) == 0 {
		return &responsesAttemptResult{
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
	// passthrough auth from the incoming request.
	hasConfiguredKeyEntries := len(ch.APIKeys) > 0
	enabledKeys := ch.EnabledAPIKeys()

	var passthroughApiKey, passthroughAuthKey string
	if !hasConfiguredKeyEntries {
		passthroughApiKey = r.Header.Get("x-api-key")
		authHeader := r.Header.Get("authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			passthroughAuthKey = strings.TrimPrefix(authHeader, "Bearer ")
		}
		log.Printf("[Responses-Auth] Passthrough mode: x-api-key=%v, auth=%v", passthroughApiKey != "", passthroughAuthKey != "")
		if passthroughApiKey == "" && passthroughAuthKey == "" {
			return &responsesAttemptResult{
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
			return &responsesAttemptResult{
				ok:           false,
				statusCode:   http.StatusUnauthorized,
				headers:      http.Header{"Content-Type": []string{"application/json"}},
				body:         jsonErrorResponse(http.StatusUnauthorized, "no enabled API keys configured for channel").body,
				errorMsg:     "no enabled API keys configured for channel",
				errorDetails: "no enabled API keys configured for channel",
			}
		}
		log.Printf("[Responses-Auth] Using channel configured API key(s): enabled=%d total=%d", len(enabledKeys), len(ch.APIKeys))
	}

	// Convert to Claude Messages format
	maxTokens := req.MaxOutputTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	claudeReq := map[string]interface{}{
		"model":      model,
		"max_tokens": maxTokens,
		"messages":   messages,
		"stream":     req.Stream,
	}

	if req.Instructions != "" {
		claudeReq["system"] = req.Instructions
	}
	if req.Temperature != nil {
		claudeReq["temperature"] = *req.Temperature
	}

	reqBody, _ := json.Marshal(claudeReq)

	// Build upstream URL
	upstreamURL := baseURL + "/v1/messages"

	// Set auth headers based on AuthType
	authType := ch.AuthType
	if authType == "" {
		authType = config.AuthTypeAPIKey // Default for Claude channels
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	keyAttempts := []string{""}
	if hasConfiguredKeyEntries {
		keyAttempts = applyAffinityKeyOrder(enabledKeys, affinityKey)
	}

	var respBody []byte
	var successAPIKey string
	for keyIndex, key := range keyAttempts {
		attemptStart := time.Now()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)

		upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(reqBody))
		if err != nil {
			cancel()
			return &responsesAttemptResult{
				ok:           false,
				statusCode:   http.StatusInternalServerError,
				headers:      http.Header{"Content-Type": []string{"application/json"}},
				body:         jsonErrorResponse(http.StatusInternalServerError, "failed to create upstream request").body,
				errorMsg:     "failed to create upstream request",
				errorDetails: "failed to create upstream request",
			}
		}

		// Set headers
		copyHeadersForUpstreamRequest(upstreamReq.Header, r.Header)
		upstreamReq.Header.Set("Content-Type", "application/json")
		if upstreamReq.Header.Get("anthropic-version") == "" {
			upstreamReq.Header.Set("anthropic-version", "2023-06-01")
		}

		// Prevent mixing client-provided tokens with configured keys.
		if hasConfiguredKeyEntries {
			upstreamReq.Header.Del("X-Api-Key")
			upstreamReq.Header.Del("Authorization")
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

		switch authType {
		case config.AuthTypeBearer:
			authKey := keyForAuth
			if authKey == "" {
				authKey = keyForApiKey
			}
			upstreamReq.Header.Set("Authorization", "Bearer "+authKey)
			log.Printf("[Responses-Auth] Sending Bearer token only")
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
			upstreamReq.Header.Set("Authorization", "Bearer "+authKey)
			log.Printf("[Responses-Auth] Sending both x-api-key and Bearer token")
		default: // AuthTypeAPIKey
			apiKey := keyForApiKey
			if apiKey == "" {
				apiKey = keyForAuth
			}
			upstreamReq.Header.Set("x-api-key", apiKey)
			log.Printf("[Responses-Auth] Sending x-api-key only")
		}

		resp, err := client.Do(upstreamReq)
		if err != nil {
			cancel()
			log.Printf("[Responses-Error] Upstream request failed: channel=%s url=%s err=%v", ch.ID, redactURLForLogs(upstreamURL), err)
			return &responsesAttemptResult{
				ok:           false,
				statusCode:   http.StatusBadGateway,
				headers:      http.Header{"Content-Type": []string{"application/json"}},
				body:         jsonErrorResponse(http.StatusBadGateway, "upstream request failed").body,
				errorMsg:     "upstream request failed: " + redactSecretsForLogs(err.Error()),
				errorDetails: "upstream request failed: " + redactSecretsForLogs(err.Error()),
			}
		}

		bodyBytes, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		if readErr != nil {
			return &responsesAttemptResult{
				ok:           false,
				statusCode:   http.StatusBadGateway,
				headers:      http.Header{"Content-Type": []string{"application/json"}},
				body:         jsonErrorResponse(http.StatusBadGateway, "failed to read upstream response").body,
				errorMsg:     "failed to read upstream response",
				errorDetails: "failed to read upstream response",
			}
		}

		if resp.StatusCode >= 400 {
			normalizedHeaders, normalizedBody := normalizeUpstreamErrorResponse(resp.StatusCode, resp.Header.Clone(), bodyBytes)
			errMsg := extractErrorMessage(normalizedBody)
			if strings.TrimSpace(errMsg) == "" {
				errMsg = "upstream returned error"
			}
			errorDetails := bodySnippetForLogs(bodyBytes, 8192)
			log.Printf("[Responses-Error] Upstream returned %d: channel=%s url=%s body=%s", resp.StatusCode, ch.ID, redactURLForLogs(upstreamURL), bodySnippetForLogs(bodyBytes, 2048))
			if hasConfiguredKeyEntries && keyIndex < len(keyAttempts)-1 && isRetryableWithAnotherAPIKey(resp.StatusCode) {
				if historyManager != nil {
					historyManager.RecordRequest(
						ch.ID,
						"responses",
						model,
						false,
						time.Since(attemptStart).Milliseconds(),
						0,
						0,
						fmt.Sprintf("API key %d/%d failed: HTTP %d: %s", keyIndex+1, len(keyAttempts), resp.StatusCode, errMsg),
						errorDetails,
					)
				}
				log.Printf("[Responses-Auth] Upstream returned %d; trying next API key (%d/%d)", resp.StatusCode, keyIndex+2, len(keyAttempts))
				continue
			}
			return &responsesAttemptResult{
				ok:           false,
				statusCode:   resp.StatusCode,
				headers:      normalizedHeaders,
				body:         normalizedBody,
				errorMsg:     fmt.Sprintf("HTTP %d: %s", resp.StatusCode, errMsg),
				errorDetails: errorDetails,
			}
		}

		respBody = bodyBytes
		successAPIKey = apiKeyUsed
		break
	}

	if respBody == nil {
		return &responsesAttemptResult{
			ok:           false,
			statusCode:   http.StatusBadGateway,
			headers:      http.Header{"Content-Type": []string{"application/json"}},
			body:         jsonErrorResponse(http.StatusBadGateway, "upstream request failed").body,
			errorMsg:     "upstream request failed",
			errorDetails: "upstream request failed",
		}
	}

	// Parse Claude response
	var claudeResp struct {
		ID      string `json:"id"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens       int64 `json:"input_tokens"`
			OutputTokens      int64 `json:"output_tokens"`
			CacheReadTokens   int64 `json:"cache_read_input_tokens"`
			CacheCreateTokens int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		parseDetails := bodySnippetForLogs(respBody, 8192)
		return &responsesAttemptResult{
			ok:           false,
			statusCode:   http.StatusBadGateway,
			headers:      http.Header{"Content-Type": []string{"application/json"}},
			body:         jsonErrorResponse(http.StatusBadGateway, "failed to parse upstream response").body,
			errorMsg:     "failed to parse upstream response",
			errorDetails: parseDetails,
		}
	}

	// Extract text content
	var textContent string
	for _, c := range claudeResp.Content {
		if c.Type == "text" {
			textContent = c.Text
			break
		}
	}

	// Store in session
	responseID, _ := sessionMgr.AddMessage(sess.ID, "assistant", textContent)

	// Build Responses API format response
	responsesResp := map[string]interface{}{
		"id":     responseID,
		"object": "response",
		"output": []map[string]interface{}{
			{
				"type": "message",
				"role": "assistant",
				"content": []map[string]interface{}{
					{
						"type": "output_text",
						"text": textContent,
					},
				},
			},
		},
		"usage": map[string]interface{}{
			"input_tokens":  claudeResp.Usage.InputTokens,
			"output_tokens": claudeResp.Usage.OutputTokens,
		},
	}

	respBytes, err := json.Marshal(responsesResp)
	if err != nil {
		return &responsesAttemptResult{
			ok:           false,
			statusCode:   http.StatusInternalServerError,
			headers:      http.Header{"Content-Type": []string{"application/json"}},
			body:         jsonErrorResponse(http.StatusInternalServerError, "failed to encode response").body,
			errorMsg:     "failed to encode response",
			errorDetails: "failed to encode response",
		}
	}

	return &responsesAttemptResult{
		ok:           true,
		statusCode:   http.StatusOK,
		headers:      http.Header{"Content-Type": []string{"application/json"}},
		body:         respBytes,
		responseID:   responseID,
		apiKeyUsed:   successAPIKey,
		inputTokens:  claudeResp.Usage.InputTokens,
		outputTokens: claudeResp.Usage.OutputTokens,
		cacheRead:    claudeResp.Usage.CacheReadTokens,
		cacheCreate:  claudeResp.Usage.CacheCreateTokens,
	}
}

func proxyOpenAIResponsesRequest(
	r *http.Request,
	requestID string,
	ch *config.Channel,
	body []byte,
	model string,
	stream bool,
	historyManager *history.Manager,
	affinityKey string,
) *responsesAttemptResult {
	baseURLs := ch.GetAllBaseURLs()
	if len(baseURLs) == 0 {
		return &responsesAttemptResult{
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
	// passthrough auth from the incoming request.
	hasConfiguredKeyEntries := len(ch.APIKeys) > 0
	enabledKeys := ch.EnabledAPIKeys()

	var passthroughApiKey, passthroughAuthKey string
	if !hasConfiguredKeyEntries {
		passthroughApiKey = r.Header.Get("x-api-key")
		authHeader := r.Header.Get("authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			passthroughAuthKey = strings.TrimPrefix(authHeader, "Bearer ")
		}
		log.Printf("[Responses-OpenAI-Auth] req=%s Passthrough mode: x-api-key=%v auth=%v", requestID, passthroughApiKey != "", passthroughAuthKey != "")
		if passthroughApiKey == "" && passthroughAuthKey == "" {
			return &responsesAttemptResult{
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
			return &responsesAttemptResult{
				ok:           false,
				statusCode:   http.StatusUnauthorized,
				headers:      http.Header{"Content-Type": []string{"application/json"}},
				body:         jsonErrorResponse(http.StatusUnauthorized, "no enabled API keys configured for channel").body,
				errorMsg:     "no enabled API keys configured for channel",
				errorDetails: "no enabled API keys configured for channel",
			}
		}
		log.Printf("[Responses-OpenAI-Auth] req=%s Using channel configured API key(s): enabled=%d total=%d", requestID, len(enabledKeys), len(ch.APIKeys))
	}

	// Modify request body with mapped model
	modifiedBody := body
	var reqBody map[string]interface{}
	if json.Unmarshal(body, &reqBody) == nil {
		reqBody["model"] = model
		if newBody, err := json.Marshal(reqBody); err == nil {
			modifiedBody = newBody
		}
	}

	upstreamURL := buildOpenAICompatibleURL(baseURL, "/responses")

	// Default auth type for OpenAI-style upstreams is bearer.
	authType := ch.AuthType
	if authType == "" {
		authType = config.AuthTypeBearer
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	keyAttempts := []string{""}
	if hasConfiguredKeyEntries {
		keyAttempts = applyAffinityKeyOrder(enabledKeys, affinityKey)
	}

	for keyIndex, key := range keyAttempts {
		attemptStart := time.Now()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(modifiedBody))
		if err != nil {
			cancel()
			return &responsesAttemptResult{
				ok:           false,
				statusCode:   http.StatusInternalServerError,
				headers:      http.Header{"Content-Type": []string{"application/json"}},
				body:         jsonErrorResponse(http.StatusInternalServerError, "failed to create upstream request").body,
				errorMsg:     "failed to create upstream request",
				errorDetails: "failed to create upstream request",
			}
		}

		copyHeadersForUpstreamRequest(upstreamReq.Header, r.Header)
		upstreamReq.Header.Set("Content-Type", "application/json")
		if stream {
			if !strings.Contains(strings.ToLower(upstreamReq.Header.Get("accept")), "text/event-stream") {
				upstreamReq.Header.Set("Accept", "text/event-stream")
			}
		}

		// Prevent mixing client-provided tokens with configured keys.
		if hasConfiguredKeyEntries {
			upstreamReq.Header.Del("X-Api-Key")
			upstreamReq.Header.Del("Authorization")
		}

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

		switch authType {
		case config.AuthTypeBearer:
			authKey := keyForAuth
			if authKey == "" {
				authKey = keyForApiKey
			}
			upstreamReq.Header.Set("Authorization", "Bearer "+authKey)
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
			upstreamReq.Header.Set("Authorization", "Bearer "+authKey)
		default: // AuthTypeAPIKey
			apiKey := keyForApiKey
			if apiKey == "" {
				apiKey = keyForAuth
			}
			upstreamReq.Header.Set("x-api-key", apiKey)
		}

		resp, err := client.Do(upstreamReq)
		if err != nil {
			cancel()
			log.Printf("[Responses-OpenAI] req=%s upstream error: channel=%s url=%s err=%v", requestID, ch.ID, redactURLForLogs(upstreamURL), err)
			return &responsesAttemptResult{
				ok:           false,
				statusCode:   http.StatusBadGateway,
				headers:      http.Header{"Content-Type": []string{"application/json"}},
				body:         jsonErrorResponse(http.StatusBadGateway, "upstream request failed").body,
				errorMsg:     "upstream request failed: " + redactSecretsForLogs(err.Error()),
				errorDetails: "upstream request failed: " + redactSecretsForLogs(err.Error()),
			}
		}

		if resp.StatusCode >= 400 {
			respBody, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			cancel()
			normalizedHeaders, normalizedBody := normalizeUpstreamErrorResponse(resp.StatusCode, resp.Header.Clone(), respBody)
			errMsg := extractErrorMessage(normalizedBody)
			if strings.TrimSpace(errMsg) == "" {
				errMsg = "upstream returned error"
			}
			errorDetails := bodySnippetForLogs(respBody, 8192)
			log.Printf("[Responses-OpenAI] req=%s upstream returned %d: channel=%s url=%s body=%s", requestID, resp.StatusCode, ch.ID, redactURLForLogs(upstreamURL), bodySnippetForLogs(respBody, 2048))
			if hasConfiguredKeyEntries && keyIndex < len(keyAttempts)-1 && isRetryableWithAnotherAPIKey(resp.StatusCode) {
				if historyManager != nil {
					historyManager.RecordRequest(
						ch.ID,
						"responses",
						model,
						false,
						time.Since(attemptStart).Milliseconds(),
						0,
						0,
						fmt.Sprintf("API key %d/%d failed: HTTP %d: %s", keyIndex+1, len(keyAttempts), resp.StatusCode, errMsg),
						errorDetails,
					)
				}
				continue
			}
			return &responsesAttemptResult{
				ok:           false,
				statusCode:   resp.StatusCode,
				headers:      normalizedHeaders,
				body:         normalizedBody,
				errorMsg:     fmt.Sprintf("HTTP %d: %s", resp.StatusCode, errMsg),
				errorDetails: errorDetails,
			}
		}

		result := &responsesAttemptResult{
			ok:         true,
			statusCode: resp.StatusCode,
			headers:    resp.Header.Clone(),
			apiKeyUsed: apiKeyUsed,
		}

		if stream {
			streamBody := cancelOnCloseReadCloser{
				ReadCloser: resp.Body,
				cancel:     cancel,
			}
			firstByte := make([]byte, 1)
			n, err := streamBody.Read(firstByte)
			if n == 0 && err != nil {
				_ = streamBody.Close()
				log.Printf("[Responses-OpenAI] req=%s upstream stream ended before first byte: err=%v", requestID, err)
				return &responsesAttemptResult{
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
		if err != nil {
			return &responsesAttemptResult{
				ok:           false,
				statusCode:   http.StatusBadGateway,
				headers:      http.Header{"Content-Type": []string{"application/json"}},
				body:         jsonErrorResponse(http.StatusBadGateway, "failed to read upstream response").body,
				errorMsg:     "failed to read upstream response",
				errorDetails: "failed to read upstream response",
			}
		}
		result.body = respBody

		// Parse usage if present (Responses API uses input_tokens/output_tokens).
		var respData struct {
			Usage struct {
				InputTokens  int64 `json:"input_tokens"`
				OutputTokens int64 `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(respBody, &respData) == nil {
			result.inputTokens = respData.Usage.InputTokens
			result.outputTokens = respData.Usage.OutputTokens
		}

		return result
	}

	return &responsesAttemptResult{
		ok:           false,
		statusCode:   http.StatusBadGateway,
		headers:      http.Header{"Content-Type": []string{"application/json"}},
		body:         jsonErrorResponse(http.StatusBadGateway, "upstream request failed").body,
		errorMsg:     "upstream request failed",
		errorDetails: "upstream request failed",
	}
}

// GeminiHandler handles the /v1beta/models/* endpoints
func GeminiHandler(cfg *config.Config, sched *scheduler.Scheduler, metricsManager *metrics.Manager, historyManager *history.Manager) http.HandlerFunc {
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

		model := extractGeminiModelFromPath(r.URL.Path)
		if model == "" {
			var bodyModel struct {
				Model string `json:"model"`
			}
			if json.Unmarshal(body, &bodyModel) == nil {
				model = strings.TrimSpace(bodyModel.Model)
			}
		}

		// Get user ID for affinity
		userID := extractGeminiSessionID(r.Header)
		if userID == "" {
			userID = strings.TrimSpace(r.Header.Get("x-user-id"))
		}
		isStreamRequest := strings.Contains(strings.ToLower(r.URL.Path), "streamgeneratecontent")

		// Select channel
		excludeChannels := make(map[string]bool)
		maxRetries := 3
		var lastFailure *bufferedHTTPResponse

		for attempt := 0; attempt < maxRetries; attempt++ {
			ch, err := sched.SelectChannel(channel.ChannelTypeGemini, userID, excludeChannels)
			if err != nil {
				if lastFailure != nil {
					lastFailure.writeTo(w)
					return
				}
				if attempt == maxRetries-1 {
					ErrorResponse(w, http.StatusServiceUnavailable, "no available channels for gemini endpoint")
					return
				}
				continue
			}

			startTime := time.Now()
			affinityKey, _ := sched.GetKeyAffinity(userID, ch.ID)

			// Forward to Gemini upstream
			result := proxyGeminiRequest(r, ch, body, model, isStreamRequest, historyManager, affinityKey)
			if result == nil {
				result = &geminiAttemptResult{
					ok:           false,
					statusCode:   http.StatusBadGateway,
					headers:      http.Header{"Content-Type": []string{"application/json"}},
					body:         jsonErrorResponse(http.StatusBadGateway, "upstream request failed").body,
					errorMsg:     "upstream request failed",
					errorDetails: "upstream request failed",
				}
			}

			latencyMs := time.Since(startTime).Milliseconds()
			metricsManager.RecordRequest(ch.ID, result.ok, latencyMs, result.inputTokens, result.outputTokens, result.cacheRead, 0)

			// Record to history
			if historyManager != nil {
				historyManager.RecordRequest(ch.ID, "gemini", model, result.ok, latencyMs, result.inputTokens, result.outputTokens, result.errorMsg, result.errorDetails)
			}

			if result.ok {
				sched.RecordSuccess(ch.ID)
				if userID != "" && result.apiKeyUsed != "" {
					sched.SetKeyAffinity(userID, ch.ID, result.apiKeyUsed, keyAffinityTTLForChannelType(channel.ChannelTypeGemini))
				}
				if isStreamRequest && result.stream != nil {
					copyHeadersForDownstreamResponse(w.Header(), result.headers)
					if w.Header().Get("Content-Type") == "" {
						w.Header().Set("Content-Type", "text/event-stream")
					}
					w.WriteHeader(result.statusCode)
					defer result.stream.Close()

					flusher, ok := w.(http.Flusher)
					if ok {
						flusher.Flush()
					}

					buf := make([]byte, 4096)
					for {
						n, err := result.stream.Read(buf)
						if n > 0 {
							_, _ = w.Write(buf[:n])
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
					return
				}

				(&bufferedHTTPResponse{
					statusCode: result.statusCode,
					headers:    result.headers,
					body:       result.body,
				}).writeTo(w)
				return
			}

			sched.RecordFailure(ch.ID, isRetryableHTTPStatus(result.statusCode))
			excludeChannels[ch.ID] = true
			lastFailure = &bufferedHTTPResponse{
				statusCode: result.statusCode,
				headers:    result.headers,
				body:       result.body,
			}
			log.Printf("[Gemini-Failover] Channel %s failed, trying next", ch.Name)
		}

		if lastFailure != nil {
			lastFailure.writeTo(w)
			return
		}
		ErrorResponse(w, http.StatusBadGateway, "all channels failed")
	})
}

// proxyGeminiRequest forwards request to Gemini upstream
func proxyGeminiRequest(
	r *http.Request,
	ch *config.Channel,
	body []byte,
	model string,
	stream bool,
	historyManager *history.Manager,
	affinityKey string,
) *geminiAttemptResult {
	baseURLs := ch.GetAllBaseURLs()
	if len(baseURLs) == 0 {
		return &geminiAttemptResult{
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
	// passthrough auth from the incoming request.
	hasConfiguredKeyEntries := len(ch.APIKeys) > 0
	enabledKeys := ch.EnabledAPIKeys()

	passthroughApiKey := ""
	if !hasConfiguredKeyEntries {
		passthroughApiKey = strings.TrimSpace(r.Header.Get(config.AuthTypeGoogAPIKey))
		if passthroughApiKey == "" {
			passthroughApiKey = strings.TrimSpace(r.Header.Get("x-api-key"))
		}
		if passthroughApiKey == "" {
			authHeader := strings.TrimSpace(r.Header.Get("authorization"))
			if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				passthroughApiKey = strings.TrimSpace(authHeader[7:])
			}
		}
		if passthroughApiKey == "" {
			return &geminiAttemptResult{
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
			return &geminiAttemptResult{
				ok:           false,
				statusCode:   http.StatusUnauthorized,
				headers:      http.Header{"Content-Type": []string{"application/json"}},
				body:         jsonErrorResponse(http.StatusUnauthorized, "no enabled API keys configured for channel").body,
				errorMsg:     "no enabled API keys configured for channel",
				errorDetails: "no enabled API keys configured for channel",
			}
		}
	}

	// Build upstream URL preserving path
	rawQuery := r.URL.RawQuery
	if hasConfiguredKeyEntries {
		if strippedQuery, removedKeys := stripAuthQueryParams(rawQuery); len(removedKeys) > 0 {
			log.Printf("[Gemini-Auth] Stripped auth query params from client request: channel=%s keys=%v", ch.ID, removedKeys)
			rawQuery = strippedQuery
		}
	}
	upstreamURL := buildGeminiCompatibleURL(baseURL, r.URL.Path, rawQuery)

	authType := strings.ToLower(strings.TrimSpace(ch.AuthType))
	if authType == "" {
		authType = config.AuthTypeGoogAPIKey
	}

	keyAttempts := []string{passthroughApiKey}
	if hasConfiguredKeyEntries {
		keyAttempts = applyAffinityKeyOrder(enabledKeys, affinityKey)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	for keyIndex, apiKey := range keyAttempts {
		attemptStart := time.Now()
		apiKeyUsed := apiKey
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)

		upstreamReq, err := http.NewRequestWithContext(ctx, r.Method, upstreamURL, bytes.NewReader(body))
		if err != nil {
			cancel()
			return &geminiAttemptResult{
				ok:           false,
				statusCode:   http.StatusInternalServerError,
				headers:      http.Header{"Content-Type": []string{"application/json"}},
				body:         jsonErrorResponse(http.StatusInternalServerError, "failed to create upstream request").body,
				errorMsg:     "failed to create upstream request",
				errorDetails: "failed to create upstream request",
			}
		}

		copyHeadersForUpstreamRequest(upstreamReq.Header, r.Header)
		upstreamReq.Header.Set("Content-Type", "application/json")

		// Explicitly control authentication headers.
		upstreamReq.Header.Del("authorization")
		upstreamReq.Header.Del("x-api-key")
		upstreamReq.Header.Del(config.AuthTypeGoogAPIKey)

		switch authType {
		case config.AuthTypeGoogAPIKey:
			upstreamReq.Header.Set(config.AuthTypeGoogAPIKey, apiKey)
		case config.AuthTypeAPIKey:
			upstreamReq.Header.Set("x-api-key", apiKey)
		case config.AuthTypeBearer:
			upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
		case config.AuthTypeBoth:
			// For Gemini, "both" means x-goog-api-key + Bearer.
			upstreamReq.Header.Set(config.AuthTypeGoogAPIKey, apiKey)
			upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
		default:
			upstreamReq.Header.Set(config.AuthTypeGoogAPIKey, apiKey)
		}

		resp, err := client.Do(upstreamReq)
		if err != nil {
			cancel()
			log.Printf("[Gemini-Error] Upstream request failed: %v", err)
			return &geminiAttemptResult{
				ok:           false,
				statusCode:   http.StatusBadGateway,
				headers:      http.Header{"Content-Type": []string{"application/json"}},
				body:         jsonErrorResponse(http.StatusBadGateway, "upstream request failed").body,
				errorMsg:     "upstream request failed: " + redactSecretsForLogs(err.Error()),
				errorDetails: "upstream request failed: " + redactSecretsForLogs(err.Error()),
			}
		}

		// Check status
		if resp.StatusCode >= 400 {
			respBody, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			cancel()
			normalizedHeaders, normalizedBody := normalizeUpstreamErrorResponse(resp.StatusCode, resp.Header.Clone(), respBody)
			errMsg := extractErrorMessage(normalizedBody)
			if strings.TrimSpace(errMsg) == "" {
				errMsg = "upstream returned error"
			}
			errorDetails := bodySnippetForLogs(respBody, 8192)
			log.Printf("[Gemini-Error] Upstream returned %d: channel=%s url=%s body=%s", resp.StatusCode, ch.ID, redactURLForLogs(upstreamURL), bodySnippetForLogs(respBody, 2048))
			if hasConfiguredKeyEntries && keyIndex < len(keyAttempts)-1 && isRetryableWithAnotherAPIKey(resp.StatusCode) {
				if historyManager != nil {
					historyManager.RecordRequest(
						ch.ID,
						"gemini",
						model,
						false,
						time.Since(attemptStart).Milliseconds(),
						0,
						0,
						fmt.Sprintf("API key %d/%d failed: HTTP %d: %s", keyIndex+1, len(keyAttempts), resp.StatusCode, errMsg),
						errorDetails,
					)
				}
				continue
			}
			return &geminiAttemptResult{
				ok:           false,
				statusCode:   resp.StatusCode,
				headers:      normalizedHeaders,
				body:         normalizedBody,
				errorMsg:     fmt.Sprintf("HTTP %d: %s", resp.StatusCode, errMsg),
				errorDetails: errorDetails,
			}
		}

		result := &geminiAttemptResult{
			ok:         true,
			statusCode: resp.StatusCode,
			headers:    resp.Header.Clone(),
			apiKeyUsed: apiKeyUsed,
		}

		if stream {
			result.stream = cancelOnCloseReadCloser{
				ReadCloser: resp.Body,
				cancel:     cancel,
			}
			return result
		}

		respBody, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		if err != nil {
			return &geminiAttemptResult{
				ok:           false,
				statusCode:   http.StatusBadGateway,
				headers:      http.Header{"Content-Type": []string{"application/json"}},
				body:         jsonErrorResponse(http.StatusBadGateway, "failed to read upstream response").body,
				errorMsg:     "failed to read upstream response",
				errorDetails: "failed to read upstream response",
			}
		}

		result.body = respBody

		var respData struct {
			UsageMetadata struct {
				PromptTokenCount        int64 `json:"promptTokenCount"`
				CandidatesTokenCount    int64 `json:"candidatesTokenCount"`
				CachedContentTokenCount int64 `json:"cachedContentTokenCount"`
			} `json:"usageMetadata"`
		}
		if json.Unmarshal(respBody, &respData) == nil {
			result.inputTokens = respData.UsageMetadata.PromptTokenCount
			result.outputTokens = respData.UsageMetadata.CandidatesTokenCount
			result.cacheRead = respData.UsageMetadata.CachedContentTokenCount
		}

		return result
	}

	return &geminiAttemptResult{
		ok:           false,
		statusCode:   http.StatusBadGateway,
		headers:      http.Header{"Content-Type": []string{"application/json"}},
		body:         jsonErrorResponse(http.StatusBadGateway, "upstream request failed").body,
		errorMsg:     "upstream request failed",
		errorDetails: "upstream request failed",
	}
}
