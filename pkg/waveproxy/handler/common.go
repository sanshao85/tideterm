// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Package handler provides HTTP handlers for the proxy service.
package handler

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sanshao85/tideterm/pkg/waveproxy/channel"
	"github.com/sanshao85/tideterm/pkg/waveproxy/config"
)

type bufferedHTTPResponse struct {
	statusCode int
	headers    http.Header
	body       []byte
}

func (r *bufferedHTTPResponse) writeTo(w http.ResponseWriter) {
	if r == nil {
		return
	}
	copyHeadersForDownstreamResponse(w.Header(), r.headers)
	if r.body != nil {
		w.Header().Set("Content-Length", strconv.Itoa(len(r.body)))
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(r.statusCode)
	_, _ = w.Write(r.body)
}

func jsonErrorResponse(statusCode int, message string) *bufferedHTTPResponse {
	body, _ := json.Marshal(map[string]interface{}{
		"error": map[string]interface{}{
			"type":    "error",
			"message": message,
		},
	})
	return &bufferedHTTPResponse{
		statusCode: statusCode,
		headers:    http.Header{"Content-Type": []string{"application/json"}},
		body:       body,
	}
}

// normalizeUpstreamErrorResponse attempts to convert non-standard upstream error payloads into our
// canonical error envelope so Claude Code and other clients display the error message consistently.
//
// Examples of non-standard error payloads we see from some providers:
//
//	{"error":"Client Not Allowed","message":"..."}
//	"Client Not Allowed"
func normalizeUpstreamErrorResponse(statusCode int, headers http.Header, body []byte) (http.Header, []byte) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return http.Header{"Content-Type": []string{"application/json"}}, jsonErrorResponse(statusCode, "upstream returned error").body
	}

	var parsed any
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		// Not JSON (or invalid JSON) -- wrap as message.
		return http.Header{"Content-Type": []string{"application/json"}}, jsonErrorResponse(statusCode, string(trimmed)).body
	}

	switch v := parsed.(type) {
	case string:
		// JSON string payload -- use the decoded value.
		if strings.TrimSpace(v) == "" {
			return http.Header{"Content-Type": []string{"application/json"}}, jsonErrorResponse(statusCode, "upstream returned error").body
		}
		return http.Header{"Content-Type": []string{"application/json"}}, jsonErrorResponse(statusCode, v).body
	case map[string]any:
		obj := v

		// If upstream already uses an error envelope (error object), pass through.
		if _, isObj := obj["error"].(map[string]any); isObj {
			return headers, body
		}

		// Common non-standard formats: error/message strings.
		if errStr, isStr := obj["error"].(string); isStr && strings.TrimSpace(errStr) != "" {
			msg := errStr
			if msgStr, ok := obj["message"].(string); ok && strings.TrimSpace(msgStr) != "" {
				msg = msgStr
			}
			return http.Header{"Content-Type": []string{"application/json"}}, jsonErrorResponse(statusCode, msg).body
		}
		if msgStr, ok := obj["message"].(string); ok && strings.TrimSpace(msgStr) != "" {
			return http.Header{"Content-Type": []string{"application/json"}}, jsonErrorResponse(statusCode, msgStr).body
		}

		return headers, body
	default:
		// Unknown JSON payload shape.
		return http.Header{"Content-Type": []string{"application/json"}}, jsonErrorResponse(statusCode, string(trimmed)).body
	}
}

var openAIVersionSuffixPattern = regexp.MustCompile(`/v\d+[a-z]*$`)
var claudeSessionIDPattern = regexp.MustCompile(`(?i)session_([a-f0-9-]+)`)

var sensitiveTokenPattern = regexp.MustCompile(`(?i)\b(bearer)\s+([a-z0-9._=-]{8,})`)
var sensitiveSkPattern = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}`)

const (
	keyAffinityTTLClaude = 5 * time.Minute
	keyAffinityTTLCodex  = 15 * time.Minute
	keyAffinityTTLGemini = 15 * time.Minute
)

func redactSecretsForLogs(text string) string {
	if text == "" {
		return ""
	}
	text = sensitiveTokenPattern.ReplaceAllString(text, "$1 REDACTED")
	text = sensitiveSkPattern.ReplaceAllString(text, "sk-REDACTED")
	return text
}

func stripAuthQueryParams(rawQuery string) (string, []string) {
	rawQuery = strings.TrimSpace(rawQuery)
	if rawQuery == "" {
		return "", nil
	}
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery, nil
	}
	var removedKeys []string
	for _, key := range []string{"key", "api_key", "apikey", "access_token", "token", "auth"} {
		if q.Has(key) {
			q.Del(key)
			removedKeys = append(removedKeys, key)
		}
	}
	return q.Encode(), removedKeys
}

func truncateForHistory(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	if text == "" || maxRunes <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "..."
}

func redactURLForLogs(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return redactSecretsForLogs(rawURL)
	}
	if parsed.User != nil {
		parsed.User = url.User("REDACTED")
	}
	if parsed.RawQuery != "" {
		q := parsed.Query()
		for _, key := range []string{"key", "api_key", "apikey", "access_token", "token", "auth"} {
			if q.Has(key) {
				q.Set(key, "REDACTED")
			}
		}
		parsed.RawQuery = q.Encode()
	}
	return redactSecretsForLogs(parsed.String())
}

func bodySnippetForLogs(body []byte, maxBytes int) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ""
	}
	truncated := false
	if maxBytes > 0 && len(trimmed) > maxBytes {
		trimmed = trimmed[:maxBytes]
		truncated = true
	}
	trimmed = bytes.ToValidUTF8(trimmed, []byte("?"))
	out := redactSecretsForLogs(string(trimmed))
	if truncated {
		out += "...(truncated)"
	}
	return out
}

func extractErrorMessage(body []byte) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ""
	}

	var parsed any
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		return truncateForHistory(redactSecretsForLogs(string(trimmed)), 500)
	}

	switch v := parsed.(type) {
	case string:
		return truncateForHistory(redactSecretsForLogs(strings.TrimSpace(v)), 500)
	case map[string]any:
		// Canonical envelope: {"error": {"message": "..."}}
		if errObj, ok := v["error"].(map[string]any); ok {
			if msg, ok := errObj["message"].(string); ok && strings.TrimSpace(msg) != "" {
				return truncateForHistory(redactSecretsForLogs(strings.TrimSpace(msg)), 500)
			}
			if msg, ok := errObj["error"].(string); ok && strings.TrimSpace(msg) != "" {
				return truncateForHistory(redactSecretsForLogs(strings.TrimSpace(msg)), 500)
			}
			if msg, ok := errObj["type"].(string); ok && strings.TrimSpace(msg) != "" {
				return truncateForHistory(redactSecretsForLogs(strings.TrimSpace(msg)), 500)
			}
		}
		// Common non-standard: {"message": "..."} or {"error": "..."}
		if msg, ok := v["message"].(string); ok && strings.TrimSpace(msg) != "" {
			return truncateForHistory(redactSecretsForLogs(strings.TrimSpace(msg)), 500)
		}
		if msg, ok := v["error"].(string); ok && strings.TrimSpace(msg) != "" {
			return truncateForHistory(redactSecretsForLogs(strings.TrimSpace(msg)), 500)
		}
	}

	return truncateForHistory(redactSecretsForLogs(string(trimmed)), 500)
}

func buildOpenAICompatibleURL(baseURL string, endpoint string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return endpoint
	}

	skipVersionPrefix := strings.HasSuffix(baseURL, "#")
	if skipVersionPrefix {
		baseURL = strings.TrimSuffix(baseURL, "#")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	hasVersionSuffix := openAIVersionSuffixPattern.MatchString(baseURL)
	if hasVersionSuffix || skipVersionPrefix {
		return baseURL + endpoint
	}
	return baseURL + "/v1" + endpoint
}

func buildGeminiCompatibleURL(baseURL string, requestPath string, rawQuery string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return requestPath
	}

	baseURL = strings.TrimSuffix(baseURL, "/")

	path := requestPath
	if strings.HasSuffix(baseURL, "/v1beta") && strings.HasPrefix(path, "/v1beta/") {
		path = strings.TrimPrefix(path, "/v1beta")
	}

	upstreamURL := baseURL + path
	if rawQuery == "" {
		return upstreamURL
	}

	separator := "?"
	if strings.Contains(upstreamURL, "?") {
		separator = "&"
	}
	return upstreamURL + separator + rawQuery
}

var hopByHopHeaders = map[string]struct{}{
	"connection":          {},
	"proxy-connection":    {},
	"keep-alive":          {},
	"proxy-authenticate":  {},
	"proxy-authorization": {},
	"te":                  {},
	"trailer":             {},
	"transfer-encoding":   {},
	"upgrade":             {},
}

func copyHeadersForUpstreamRequest(dst http.Header, src http.Header) {
	for key, values := range src {
		lower := strings.ToLower(key)
		if _, ok := hopByHopHeaders[lower]; ok {
			continue
		}
		switch lower {
		case "host", "content-length":
			continue
		// Avoid passing through Accept-Encoding. If we forward it, Go's HTTP transport
		// will not transparently decode gzipped upstream responses, and some clients
		// (notably Claude Code's non-streaming fallback) can fail to decode/parse the
		// compressed JSON.
		case "accept-encoding":
			continue
		// We explicitly control auth headers per channel.
		case "authorization", "x-api-key":
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func copyHeadersForDownstreamResponse(dst http.Header, src http.Header) {
	for key, values := range src {
		lower := strings.ToLower(key)
		if _, ok := hopByHopHeaders[lower]; ok {
			continue
		}
		if lower == "content-length" {
			continue
		}
		// Avoid accidentally duplicating headers across retries.
		dst.Del(key)
		dst[key] = append([]string(nil), values...)
	}
}

// HealthHandler returns a health check handler
func HealthHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status":  "healthy",
			"service": "waveproxy",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// AuthMiddleware validates access key
func AuthMiddleware(cfg *config.Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Skip auth if no access key configured
		if cfg.AccessKey == "" {
			next(w, r)
			return
		}

		// Check API key header
		apiKey := r.Header.Get("x-api-key")
		if apiKey == "" {
			apiKey = r.Header.Get("Authorization")
			if len(apiKey) > 7 && apiKey[:7] == "Bearer " {
				apiKey = apiKey[7:]
			}
		}

		if apiKey != cfg.AccessKey {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

// ErrorResponse sends an error response
func ErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"type":    "error",
			"message": message,
		},
	})
}

func NotFoundHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[WaveProxy] 404 %s %s", r.Method, r.URL.Path)
		ErrorResponse(w, http.StatusNotFound, "not found")
	}
}

func isRetryableHTTPStatus(statusCode int) bool {
	if statusCode >= 500 {
		return true
	}
	switch statusCode {
	case http.StatusTooManyRequests, http.StatusRequestTimeout, http.StatusTooEarly:
		return true
	default:
		return false
	}
}

func isRetryableWithAnotherAPIKey(statusCode int) bool {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests:
		return true
	default:
		return false
	}
}

func keyAffinityTTLForChannelType(channelType channel.ChannelType) time.Duration {
	switch channelType {
	case channel.ChannelTypeMessages:
		return keyAffinityTTLClaude
	case channel.ChannelTypeGemini:
		return keyAffinityTTLGemini
	case channel.ChannelTypeResponses:
		return keyAffinityTTLCodex
	default:
		return keyAffinityTTLCodex
	}
}

func applyAffinityKeyOrder(keys []string, affinityKey string) []string {
	affinityKey = strings.TrimSpace(affinityKey)
	if affinityKey == "" || len(keys) <= 1 {
		return keys
	}
	index := -1
	for i, key := range keys {
		if key == affinityKey {
			index = i
			break
		}
	}
	if index <= 0 {
		return keys
	}
	ordered := make([]string, 0, len(keys))
	ordered = append(ordered, affinityKey)
	for i, key := range keys {
		if i == index {
			continue
		}
		ordered = append(ordered, key)
	}
	return ordered
}

func extractClaudeCodeSessionID(metadata json.RawMessage) string {
	if len(metadata) == 0 {
		return ""
	}
	var meta struct {
		UserID string `json:"user_id"`
	}
	if json.Unmarshal(metadata, &meta) != nil {
		return ""
	}
	if meta.UserID == "" {
		return ""
	}
	match := claudeSessionIDPattern.FindStringSubmatch(meta.UserID)
	if len(match) < 2 {
		return ""
	}
	return "claude_" + match[1]
}

func extractGeminiSessionID(headers http.Header) string {
	if headers == nil {
		return ""
	}
	userID := strings.TrimSpace(headers.Get("x-gemini-api-privileged-user-id"))
	if userID == "" {
		return ""
	}
	return "gemini_" + userID
}

func extractGeminiModelFromPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	const prefix = "/v1beta/models/"
	if strings.HasPrefix(path, prefix) {
		path = strings.TrimPrefix(path, prefix)
	} else if idx := strings.Index(path, prefix); idx >= 0 {
		path = path[idx+len(prefix):]
	}
	if path == "" {
		return ""
	}
	if idx := strings.Index(path, "/"); idx >= 0 {
		path = path[:idx]
	}
	if idx := strings.Index(path, ":"); idx >= 0 {
		path = path[:idx]
	}
	return strings.TrimSpace(path)
}
