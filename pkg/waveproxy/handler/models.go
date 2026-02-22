// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/sanshao85/tideterm/pkg/waveproxy/channel"
	"github.com/sanshao85/tideterm/pkg/waveproxy/config"
	"github.com/sanshao85/tideterm/pkg/waveproxy/scheduler"
)

func ModelsHandler(cfg *config.Config, sched *scheduler.Scheduler) http.HandlerFunc {
	return AuthMiddleware(cfg, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			ErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		userID := r.Header.Get("x-user-id")
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
					ErrorResponse(w, http.StatusServiceUnavailable, "no available channels for models endpoint")
					return
				}
				continue
			}

			// Models endpoint is OpenAI-compatible. Skip non-OpenAI channels.
			if strings.TrimSpace(ch.ServiceType) != "openai" {
				excludeChannels[ch.ID] = true
				continue
			}

			result := proxyOpenAIModelsRequest(r, ch, "")
			if result == nil {
				result = &bufferedHTTPResponse{
					statusCode: http.StatusBadGateway,
					headers:    http.Header{"Content-Type": []string{"application/json"}},
					body:       jsonErrorResponse(http.StatusBadGateway, "upstream request failed").body,
				}
			}

			if result.statusCode < 400 {
				result.writeTo(w)
				return
			}

			excludeChannels[ch.ID] = true
			lastFailure = result
			log.Printf("[Models-Failover] Channel %s failed, trying next", ch.Name)
		}

		if lastFailure != nil {
			lastFailure.writeTo(w)
			return
		}
		ErrorResponse(w, http.StatusBadGateway, "all channels failed")
	})
}

func ModelsDetailHandler(cfg *config.Config, sched *scheduler.Scheduler) http.HandlerFunc {
	return AuthMiddleware(cfg, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			ErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		path := r.URL.Path
		modelID := ""
		if strings.HasPrefix(path, "/v1/models/") {
			modelID = strings.TrimPrefix(path, "/v1/models/")
		} else if strings.HasPrefix(path, "/models/") {
			modelID = strings.TrimPrefix(path, "/models/")
		}
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			ErrorResponse(w, http.StatusBadRequest, "model id is required")
			return
		}

		userID := r.Header.Get("x-user-id")
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
					ErrorResponse(w, http.StatusServiceUnavailable, "no available channels for models endpoint")
					return
				}
				continue
			}

			if strings.TrimSpace(ch.ServiceType) != "openai" {
				excludeChannels[ch.ID] = true
				continue
			}

			result := proxyOpenAIModelsRequest(r, ch, "/"+modelID)
			if result == nil {
				result = &bufferedHTTPResponse{
					statusCode: http.StatusBadGateway,
					headers:    http.Header{"Content-Type": []string{"application/json"}},
					body:       jsonErrorResponse(http.StatusBadGateway, "upstream request failed").body,
				}
			}

			if result.statusCode < 400 {
				result.writeTo(w)
				return
			}

			excludeChannels[ch.ID] = true
			lastFailure = result
			log.Printf("[Models-Failover] Channel %s failed, trying next", ch.Name)
		}

		if lastFailure != nil {
			lastFailure.writeTo(w)
			return
		}
		ErrorResponse(w, http.StatusBadGateway, "all channels failed")
	})
}

func proxyOpenAIModelsRequest(r *http.Request, ch *config.Channel, suffix string) *bufferedHTTPResponse {
	baseURLs := ch.GetAllBaseURLs()
	if len(baseURLs) == 0 {
		return jsonErrorResponse(http.StatusBadGateway, "no base URL configured for channel")
	}
	baseURL := baseURLs[0]
	upstreamURL := buildOpenAICompatibleURL(baseURL, "/models"+suffix)

	// Determine authentication keys (same strategy as other handlers).
	hasConfiguredKeyEntries := len(ch.APIKeys) > 0
	enabledKeys := ch.EnabledAPIKeys()
	var passthroughApiKey, passthroughAuthKey string
	if !hasConfiguredKeyEntries {
		passthroughApiKey = r.Header.Get("x-api-key")
		authHeader := r.Header.Get("authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			passthroughAuthKey = strings.TrimPrefix(authHeader, "Bearer ")
		}
		if passthroughApiKey == "" && passthroughAuthKey == "" {
			return jsonErrorResponse(http.StatusUnauthorized, "no authentication provided")
		}
	} else {
		if len(enabledKeys) == 0 {
			return jsonErrorResponse(http.StatusUnauthorized, "no enabled API keys configured for channel")
		}
	}

	// Default auth type for OpenAI-style upstreams is bearer.
	authType := ch.AuthType
	if authType == "" {
		authType = config.AuthTypeBearer
	}

	client := &http.Client{Timeout: 30 * time.Second}

	keyAttempts := []string{""}
	if hasConfiguredKeyEntries {
		keyAttempts = enabledKeys
	}

	for keyIndex, key := range keyAttempts {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)

		upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
		if err != nil {
			cancel()
			return jsonErrorResponse(http.StatusInternalServerError, "failed to create upstream request")
		}

		copyHeadersForUpstreamRequest(upstreamReq.Header, r.Header)
		upstreamReq.Header.Set("Content-Type", "application/json")

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
			log.Printf("[Models] upstream error: channel=%s url=%s err=%v", ch.ID, redactURLForLogs(upstreamURL), err)
			return jsonErrorResponse(http.StatusBadGateway, "upstream request failed")
		}

		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()

		if resp.StatusCode >= 400 {
			log.Printf("[Models] upstream returned %d: channel=%s url=%s body=%s", resp.StatusCode, ch.ID, redactURLForLogs(upstreamURL), bodySnippetForLogs(respBody, 2048))
			if hasConfiguredKeyEntries && keyIndex < len(keyAttempts)-1 && isRetryableWithAnotherAPIKey(resp.StatusCode) {
				continue
			}
			normalizedHeaders, normalizedBody := normalizeUpstreamErrorResponse(resp.StatusCode, resp.Header.Clone(), respBody)
			return &bufferedHTTPResponse{
				statusCode: resp.StatusCode,
				headers:    normalizedHeaders,
				body:       normalizedBody,
			}
		}

		return &bufferedHTTPResponse{
			statusCode: resp.StatusCode,
			headers:    resp.Header.Clone(),
			body:       respBody,
		}
	}

	return jsonErrorResponse(http.StatusBadGateway, "upstream request failed")
}
