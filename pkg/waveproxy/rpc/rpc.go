// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Package rpc provides RPC command handlers for the proxy service.
package rpc

import (
	"context"
	"fmt"
	"sync"

	"github.com/sanshao85/tideterm/pkg/waveproxy"
	"github.com/sanshao85/tideterm/pkg/waveproxy/channel"
	"github.com/sanshao85/tideterm/pkg/waveproxy/config"
)

// Command names for proxy RPC
const (
	Command_ProxyStart          = "proxystart"
	Command_ProxyStop           = "proxystop"
	Command_ProxyStatus         = "proxystatus"
	Command_ProxyReloadConfig   = "proxyreloadconfig"
	Command_ChannelList         = "channellist"
	Command_ChannelCreate       = "channelcreate"
	Command_ChannelUpdate       = "channelupdate"
	Command_ChannelDelete       = "channeldelete"
	Command_ChannelPing         = "channelping"
	Command_ChannelMetrics      = "channelmetrics"
	Command_ChannelMetricsReset = "channelmetricsreset"
	Command_GlobalStats         = "globalstats"
	Command_SchedulerStats      = "schedulerstats"
	Command_SchedulerReset      = "schedulerreset"
)

// ProxyStatusData represents proxy status response
type ProxyStatusData struct {
	Running      bool   `json:"running"`
	Port         int    `json:"port"`
	StartedAt    string `json:"startedAt,omitempty"`
	Uptime       string `json:"uptime,omitempty"`
	Version      string `json:"version"`
	ChannelCount int    `json:"channelCount"`
}

// ChannelListRequest represents a channel list request
type ChannelListRequest struct {
	ChannelType string `json:"channelType"` // messages, responses, gemini
}

// ChannelListResponse represents a channel list response
type ChannelListResponse struct {
	Channels []config.Channel `json:"channels"`
}

// ChannelCreateRequest represents a channel create request
type ChannelCreateRequest struct {
	ChannelType string         `json:"channelType"`
	Channel     config.Channel `json:"channel"`
}

// ChannelUpdateRequest represents a channel update request
type ChannelUpdateRequest struct {
	ChannelType string         `json:"channelType"`
	Index       int            `json:"index"`
	Channel     config.Channel `json:"channel"`
}

// ChannelDeleteRequest represents a channel delete request
type ChannelDeleteRequest struct {
	ChannelType string `json:"channelType"`
	Index       int    `json:"index"`
}

// ChannelPingRequest represents a channel ping request
type ChannelPingRequest struct {
	ChannelType string `json:"channelType"`
	Index       int    `json:"index"`
}

// ChannelPingResponse represents a channel ping response
type ChannelPingResponse struct {
	Success   bool   `json:"success"`
	LatencyMs int64  `json:"latencyMs"`
	Error     string `json:"error,omitempty"`
}

// ChannelMetricsRequest represents a channel metrics request
type ChannelMetricsRequest struct {
	ChannelID string `json:"channelId,omitempty"` // Empty for all channels
}

// ChannelMetricsResponse represents channel metrics
type ChannelMetricsResponse struct {
	ChannelID           string  `json:"channelId"`
	RequestCount        int64   `json:"requestCount"`
	SuccessCount        int64   `json:"successCount"`
	FailureCount        int64   `json:"failureCount"`
	SuccessRate         float64 `json:"successRate"`
	ConsecutiveFailures int64   `json:"consecutiveFailures"`
	CircuitBroken       bool    `json:"circuitBroken"`
	InputTokens         int64   `json:"inputTokens"`
	OutputTokens        int64   `json:"outputTokens"`
	CacheHitRate        float64 `json:"cacheHitRate"`
	AvgLatencyMs        float64 `json:"avgLatencyMs"`
}

// GlobalStatsResponse represents global statistics
type GlobalStatsResponse struct {
	TotalRequests int64   `json:"totalRequests"`
	SuccessCount  int64   `json:"successCount"`
	FailureCount  int64   `json:"failureCount"`
	SuccessRate   float64 `json:"successRate"`
	ChannelCount  int     `json:"channelCount"`
}

// SchedulerStatsResponse represents scheduler statistics
type SchedulerStatsResponse struct {
	CircuitBreakers map[string]map[string]interface{} `json:"circuitBreakers"`
	AffinityCount   int                               `json:"affinityCount"`
}

// SchedulerResetRequest represents a scheduler reset request
type SchedulerResetRequest struct {
	ChannelID string `json:"channelId"` // Channel to reset circuit breaker for
}

// ProxyRPCHandler handles proxy RPC commands
type ProxyRPCHandler struct {
	mu     sync.RWMutex
	server *waveproxy.ProxyServer
}

var globalHandler *ProxyRPCHandler
var handlerOnce sync.Once

// GetProxyRPCHandler returns the singleton RPC handler
func GetProxyRPCHandler() *ProxyRPCHandler {
	handlerOnce.Do(func() {
		globalHandler = &ProxyRPCHandler{}
	})
	return globalHandler
}

// SetProxyServer sets the proxy server instance
func (h *ProxyRPCHandler) SetProxyServer(server *waveproxy.ProxyServer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.server = server
}

// GetProxyServer returns the proxy server instance
func (h *ProxyRPCHandler) GetProxyServer() *waveproxy.ProxyServer {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.server
}

// ProxyStartCommand starts the proxy server
func (h *ProxyRPCHandler) ProxyStartCommand(ctx context.Context) error {
	server := h.GetProxyServer()
	if server == nil {
		// Create new server with default config
		cfg := config.DefaultConfig()
		var err error
		server, err = waveproxy.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to create proxy server: %w", err)
		}
		h.SetProxyServer(server)
	}

	return server.Start(ctx)
}

// ProxyStopCommand stops the proxy server
func (h *ProxyRPCHandler) ProxyStopCommand(ctx context.Context) error {
	server := h.GetProxyServer()
	if server == nil {
		return fmt.Errorf("proxy server not initialized")
	}
	return server.Stop(ctx)
}

// ProxyStatusCommand returns the proxy server status
func (h *ProxyRPCHandler) ProxyStatusCommand(ctx context.Context) (*ProxyStatusData, error) {
	server := h.GetProxyServer()
	if server == nil {
		return &ProxyStatusData{
			Running: false,
			Version: waveproxy.Version,
		}, nil
	}

	status := server.Status()
	return &ProxyStatusData{
		Running:      status.Running,
		Port:         status.Port,
		StartedAt:    status.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		Uptime:       status.Uptime,
		Version:      status.Version,
		ChannelCount: status.ChannelCount,
	}, nil
}

// ChannelListCommand lists channels
func (h *ProxyRPCHandler) ChannelListCommand(ctx context.Context, data ChannelListRequest) (*ChannelListResponse, error) {
	server := h.GetProxyServer()
	if server == nil {
		return &ChannelListResponse{Channels: []config.Channel{}}, nil
	}

	channelType := parseChannelType(data.ChannelType)
	channels := server.GetChannelManager().GetChannels(channelType)

	return &ChannelListResponse{Channels: channels}, nil
}

// ChannelCreateCommand creates a new channel
func (h *ProxyRPCHandler) ChannelCreateCommand(ctx context.Context, data ChannelCreateRequest) error {
	server := h.GetProxyServer()
	if server == nil {
		return fmt.Errorf("proxy server not initialized")
	}

	channelType := parseChannelType(data.ChannelType)
	return server.GetChannelManager().AddChannel(channelType, data.Channel)
}

// ChannelUpdateCommand updates a channel
func (h *ProxyRPCHandler) ChannelUpdateCommand(ctx context.Context, data ChannelUpdateRequest) error {
	server := h.GetProxyServer()
	if server == nil {
		return fmt.Errorf("proxy server not initialized")
	}

	channelType := parseChannelType(data.ChannelType)
	return server.GetChannelManager().UpdateChannel(channelType, data.Index, data.Channel)
}

// ChannelDeleteCommand deletes a channel
func (h *ProxyRPCHandler) ChannelDeleteCommand(ctx context.Context, data ChannelDeleteRequest) error {
	server := h.GetProxyServer()
	if server == nil {
		return fmt.Errorf("proxy server not initialized")
	}

	channelType := parseChannelType(data.ChannelType)
	return server.GetChannelManager().DeleteChannel(channelType, data.Index)
}

// ChannelPingCommand pings a channel
func (h *ProxyRPCHandler) ChannelPingCommand(ctx context.Context, data ChannelPingRequest) (*ChannelPingResponse, error) {
	server := h.GetProxyServer()
	if server == nil {
		return &ChannelPingResponse{Success: false, Error: "proxy server not initialized"}, nil
	}

	channelType := parseChannelType(data.ChannelType)
	ch := server.GetChannelManager().GetChannel(channelType, data.Index)
	if ch == nil {
		return &ChannelPingResponse{Success: false, Error: "channel not found"}, nil
	}

	// TODO: Implement actual ping logic
	return &ChannelPingResponse{Success: true, LatencyMs: 50}, nil
}

// ChannelMetricsCommand returns channel metrics
func (h *ProxyRPCHandler) ChannelMetricsCommand(ctx context.Context, data ChannelMetricsRequest) ([]ChannelMetricsResponse, error) {
	server := h.GetProxyServer()
	if server == nil {
		return []ChannelMetricsResponse{}, nil
	}

	metricsManager := server.GetMetricsManager()
	if data.ChannelID != "" {
		metrics := metricsManager.GetChannelMetrics(data.ChannelID)
		return []ChannelMetricsResponse{{
			ChannelID:           metrics.ChannelID,
			RequestCount:        metrics.RequestCount,
			SuccessCount:        metrics.SuccessCount,
			FailureCount:        metrics.FailureCount,
			SuccessRate:         metrics.SuccessRate,
			ConsecutiveFailures: metrics.ConsecutiveFailures,
			CircuitBroken:       metrics.CircuitBroken,
			InputTokens:         metrics.InputTokens,
			OutputTokens:        metrics.OutputTokens,
			CacheHitRate:        metrics.CacheHitRate,
			AvgLatencyMs:        metrics.AvgLatencyMs,
		}}, nil
	}

	allMetrics := metricsManager.GetAllChannelMetrics()
	result := make([]ChannelMetricsResponse, 0, len(allMetrics))
	for _, m := range allMetrics {
		result = append(result, ChannelMetricsResponse{
			ChannelID:           m.ChannelID,
			RequestCount:        m.RequestCount,
			SuccessCount:        m.SuccessCount,
			FailureCount:        m.FailureCount,
			SuccessRate:         m.SuccessRate,
			ConsecutiveFailures: m.ConsecutiveFailures,
			CircuitBroken:       m.CircuitBroken,
			InputTokens:         m.InputTokens,
			OutputTokens:        m.OutputTokens,
			CacheHitRate:        m.CacheHitRate,
			AvgLatencyMs:        m.AvgLatencyMs,
		})
	}

	return result, nil
}

// GlobalStatsCommand returns global statistics
func (h *ProxyRPCHandler) GlobalStatsCommand(ctx context.Context) (*GlobalStatsResponse, error) {
	server := h.GetProxyServer()
	if server == nil {
		return &GlobalStatsResponse{}, nil
	}

	stats := server.GetMetricsManager().GetGlobalStats()
	return &GlobalStatsResponse{
		TotalRequests: stats["totalRequests"].(int64),
		SuccessCount:  stats["successCount"].(int64),
		FailureCount:  stats["failureCount"].(int64),
		SuccessRate:   stats["successRate"].(float64),
		ChannelCount:  stats["channelCount"].(int),
	}, nil
}

// SchedulerStatsCommand returns scheduler statistics
func (h *ProxyRPCHandler) SchedulerStatsCommand(ctx context.Context) (*SchedulerStatsResponse, error) {
	server := h.GetProxyServer()
	if server == nil {
		return &SchedulerStatsResponse{}, nil
	}

	stats := server.GetScheduler().GetSchedulerStats()
	return &SchedulerStatsResponse{
		CircuitBreakers: stats["circuitBreakers"].(map[string]map[string]interface{}),
		AffinityCount:   stats["affinityCount"].(int),
	}, nil
}

// SchedulerResetCommand resets a channel's circuit breaker
func (h *ProxyRPCHandler) SchedulerResetCommand(ctx context.Context, data SchedulerResetRequest) error {
	server := h.GetProxyServer()
	if server == nil {
		return fmt.Errorf("proxy server not initialized")
	}

	server.GetScheduler().ResetCircuit(data.ChannelID)
	return nil
}

// parseChannelType converts string to ChannelType
func parseChannelType(s string) channel.ChannelType {
	switch s {
	case "messages":
		return channel.ChannelTypeMessages
	case "responses":
		return channel.ChannelTypeResponses
	case "gemini":
		return channel.ChannelTypeGemini
	default:
		return channel.ChannelTypeMessages
	}
}
