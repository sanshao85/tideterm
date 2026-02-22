// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Package waveproxy provides RPC interface functions for the proxy service.
package waveproxy

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/sanshao85/tideterm/pkg/waveproxy/config"
)

// Global proxy server instance
var (
	globalProxyServer *ProxyServer
	globalProxyConfig *config.Config
	proxyMutex        sync.RWMutex
)

func ensureProxyConfigLoadedLocked() error {
	if globalProxyConfig != nil {
		return nil
	}
	cfg, err := loadProxyConfigFromDisk()
	if err != nil {
		return err
	}
	globalProxyConfig = cfg
	return nil
}

// ChannelInfo represents channel info for RPC
type ChannelInfo struct {
	ID             string
	Name           string
	ServiceType    string
	BaseUrl        string
	BaseUrls       []string
	ApiKeys        []config.APIKey
	AuthType       string
	Priority       int
	Status         string
	PromotionUntil string
	ModelMapping   map[string]string
	LowQuality     bool
	Description    string
}

// PingResult represents ping result for RPC
type PingResult struct {
	Success   bool
	LatencyMs int64
	Error     string
}

// MetricsInfo represents metrics for RPC
type MetricsInfo struct {
	ChannelID           string
	RequestCount        int64
	SuccessCount        int64
	FailureCount        int64
	SuccessRate         float64
	ConsecutiveFailures int64
	CircuitBroken       bool
	InputTokens         int64
	OutputTokens        int64
	CacheHitRate        float64
	AvgLatencyMs        float64
}

// GlobalStatsInfo represents global stats for RPC
type GlobalStatsInfo struct {
	TotalRequests int64
	SuccessCount  int64
	FailureCount  int64
	SuccessRate   float64
	ChannelCount  int
}

// RequestHistoryRecord represents a request history record for RPC
type RequestHistoryRecord struct {
	ID           string
	Timestamp    string
	ChannelID    string
	ChannelType  string
	Model        string
	Success      bool
	LatencyMs    int64
	InputTokens  int64
	OutputTokens int64
	ErrorMsg     string
	ErrorDetails string
}

// RpcProxyStatus represents the status returned to RPC clients
type RpcProxyStatus struct {
	Running      bool
	Port         int
	StartedAt    string
	Uptime       string
	Version      string
	ChannelCount int
}

// StartProxyServer starts the global proxy server
func StartProxyServer(ctx context.Context) error {
	proxyMutex.Lock()
	defer proxyMutex.Unlock()

	if globalProxyServer != nil {
		status := globalProxyServer.Status()
		if status.Running {
			return fmt.Errorf("proxy server is already running")
		}
	}

	if err := ensureProxyConfigLoadedLocked(); err != nil {
		return err
	}

	log.Printf("[WaveProxy] Config loaded: %d channels, %d response channels, %d gemini channels",
		len(globalProxyConfig.Channels), len(globalProxyConfig.ResponseChannels), len(globalProxyConfig.GeminiChannels))
	for i, ch := range globalProxyConfig.Channels {
		log.Printf("[WaveProxy] Channel[%d]: id=%s, name=%s, status=%s, baseUrl=%s",
			i, ch.ID, ch.Name, ch.Status, ch.BaseURL)
	}

	serverCfg := globalProxyConfig.Clone()
	server, err := New(serverCfg)
	if err != nil {
		return fmt.Errorf("failed to create proxy server: %w", err)
	}

	if err := server.Start(ctx); err != nil {
		return fmt.Errorf("failed to start proxy server: %w", err)
	}

	globalProxyServer = server
	return nil
}

// StopProxyServer stops the global proxy server
func StopProxyServer(ctx context.Context) error {
	proxyMutex.Lock()
	defer proxyMutex.Unlock()

	if globalProxyServer == nil {
		return fmt.Errorf("proxy server not initialized")
	}

	if err := globalProxyServer.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop proxy server: %w", err)
	}

	return nil
}

// GetProxyStatus returns the current proxy server status
func GetProxyStatus() RpcProxyStatus {
	proxyMutex.Lock()
	defer proxyMutex.Unlock()

	if err := ensureProxyConfigLoadedLocked(); err != nil {
		return RpcProxyStatus{
			Running: false,
			Version: Version,
		}
	}

	if globalProxyServer == nil {
		return RpcProxyStatus{
			Running:      false,
			Port:         globalProxyConfig.Port,
			Version:      Version,
			ChannelCount: countConfigChannels(globalProxyConfig),
		}
	}

	status := globalProxyServer.Status()
	if !status.Running {
		// When stopped, prefer showing the configured port (not the last server's port),
		// since the config can be updated while the proxy is not running.
		return RpcProxyStatus{
			Running:      false,
			Port:         globalProxyConfig.Port,
			Version:      Version,
			ChannelCount: countConfigChannels(globalProxyConfig),
		}
	}
	startedAtStr := ""
	startedAtStr = status.StartedAt.Format(time.RFC3339)
	return RpcProxyStatus{
		Running:      status.Running,
		Port:         status.Port,
		StartedAt:    startedAtStr,
		Uptime:       status.Uptime,
		Version:      status.Version,
		ChannelCount: status.ChannelCount,
	}
}

// SetProxyPort updates the configured listen port. If the proxy is running, it will be restarted on the new port.
func SetProxyPort(ctx context.Context, port int) error {
	if ctx == nil {
		ctx = context.Background()
	}

	proxyMutex.Lock()
	defer proxyMutex.Unlock()

	if err := ensureProxyConfigLoadedLocked(); err != nil {
		return err
	}
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	if globalProxyConfig.Port == port {
		return nil
	}

	// Not running -> just persist the new port.
	if globalProxyServer == nil || !globalProxyServer.Status().Running {
		newCfg := globalProxyConfig.Clone()
		newCfg.Port = port
		if err := saveProxyConfigToDisk(newCfg); err != nil {
			return err
		}
		globalProxyConfig = newCfg
		// Best-effort: keep any existing stopped server's view of config in sync.
		if globalProxyServer != nil {
			_ = globalProxyServer.ReloadConfig(newCfg)
		}
		return nil
	}

	// Running -> start a new server first (so the old one stays up if the new port is unavailable),
	// then persist config, then swap.
	oldCfg := globalProxyConfig.Clone()
	oldServer := globalProxyServer

	newCfg := globalProxyConfig.Clone()
	newCfg.Port = port

	newServer, err := New(newCfg)
	if err != nil {
		return err
	}
	if err := newServer.Start(ctx); err != nil {
		return err
	}

	if err := saveProxyConfigToDisk(newCfg); err != nil {
		_ = newServer.Stop(ctx)
		return err
	}

	if err := oldServer.Stop(ctx); err != nil {
		_ = newServer.Stop(ctx)
		_ = saveProxyConfigToDisk(oldCfg)
		globalProxyConfig = oldCfg
		return err
	}

	globalProxyServer = newServer
	globalProxyConfig = newCfg
	return nil
}

// GetChannelList returns channels for a given type
func GetChannelList(channelType string) []*ChannelInfo {
	proxyMutex.Lock()
	if err := ensureProxyConfigLoadedLocked(); err != nil {
		proxyMutex.Unlock()
		return []*ChannelInfo{}
	}
	cfg := globalProxyConfig.Clone()
	proxyMutex.Unlock()

	channelsPtr, err := configSliceForChannelType(cfg, channelType)
	if err != nil {
		return []*ChannelInfo{}
	}
	channels := *channelsPtr

	result := make([]*ChannelInfo, len(channels))
	for i, ch := range channels {
		promotionStr := ""
		if ch.PromotionUntil != nil {
			promotionStr = ch.PromotionUntil.Format(time.RFC3339)
		}
		result[i] = &ChannelInfo{
			ID:             ch.ID,
			Name:           ch.Name,
			ServiceType:    ch.ServiceType,
			BaseUrl:        ch.BaseURL,
			BaseUrls:       ch.BaseURLs,
			ApiKeys:        ch.APIKeys,
			AuthType:       ch.AuthType,
			Priority:       ch.Priority,
			Status:         ch.Status,
			PromotionUntil: promotionStr,
			ModelMapping:   ch.ModelMapping,
			LowQuality:     ch.LowQuality,
			Description:    ch.Description,
		}
	}

	return result
}

// CreateChannel creates a new channel
func CreateChannel(channelType string, channel interface{}) error {
	proxyMutex.Lock()
	defer proxyMutex.Unlock()

	if err := ensureProxyConfigLoadedLocked(); err != nil {
		return err
	}

	ch, err := decodeProxyChannel(channel)
	if err != nil {
		return err
	}

	channels, err := configSliceForChannelType(globalProxyConfig, channelType)
	if err != nil {
		return err
	}
	*channels = append(*channels, *ch)

	if err := saveProxyConfigToDisk(globalProxyConfig); err != nil {
		return err
	}
	if globalProxyServer != nil {
		globalProxyServer.GetChannelManager().LoadChannels(globalProxyConfig)
	}
	return nil
}

// UpdateChannel updates an existing channel
func UpdateChannel(channelType string, index int, channel interface{}) error {
	proxyMutex.Lock()
	defer proxyMutex.Unlock()

	if err := ensureProxyConfigLoadedLocked(); err != nil {
		return err
	}

	ch, err := decodeProxyChannel(channel)
	if err != nil {
		return err
	}

	channels, err := configSliceForChannelType(globalProxyConfig, channelType)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(*channels) {
		return fmt.Errorf("channel index out of range")
	}

	// Preserve ID if not provided
	if ch.ID == "" {
		ch.ID = (*channels)[index].ID
	}
	(*channels)[index] = *ch

	if err := saveProxyConfigToDisk(globalProxyConfig); err != nil {
		return err
	}
	if globalProxyServer != nil {
		globalProxyServer.GetChannelManager().LoadChannels(globalProxyConfig)
	}
	return nil
}

// DeleteChannel deletes a channel
func DeleteChannel(channelType string, index int) error {
	proxyMutex.Lock()
	defer proxyMutex.Unlock()

	if err := ensureProxyConfigLoadedLocked(); err != nil {
		return err
	}

	channels, err := configSliceForChannelType(globalProxyConfig, channelType)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(*channels) {
		return fmt.Errorf("channel index out of range")
	}
	*channels = append((*channels)[:index], (*channels)[index+1:]...)

	if err := saveProxyConfigToDisk(globalProxyConfig); err != nil {
		return err
	}
	if globalProxyServer != nil {
		globalProxyServer.GetChannelManager().LoadChannels(globalProxyConfig)
	}
	return nil
}

// PingChannel tests connectivity to a channel
func PingChannel(channelType string, index int) PingResult {
	proxyMutex.Lock()
	if err := ensureProxyConfigLoadedLocked(); err != nil {
		proxyMutex.Unlock()
		return PingResult{Success: false, Error: err.Error()}
	}
	cfg := globalProxyConfig.Clone()
	proxyMutex.Unlock()

	channels, err := configSliceForChannelType(cfg, channelType)
	if err != nil {
		return PingResult{Success: false, Error: err.Error()}
	}
	if index < 0 || index >= len(*channels) {
		return PingResult{Success: false, Error: "channel index out of range"}
	}

	ch := (*channels)[index]
	baseURLs := ch.GetAllBaseURLs()
	if len(baseURLs) == 0 {
		return PingResult{Success: false, Error: "no base URL configured"}
	}

	latency, err := pingBaseURL(baseURLs[0], ch.InsecureSkipVerify)
	if err != nil {
		return PingResult{Success: false, Error: err.Error()}
	}
	return PingResult{Success: true, LatencyMs: latency}
}

// GetMetrics returns channel metrics
func GetMetrics(channelID string) []*MetricsInfo {
	proxyMutex.RLock()
	defer proxyMutex.RUnlock()

	if globalProxyServer == nil {
		return []*MetricsInfo{}
	}

	metricsManager := globalProxyServer.GetMetricsManager()
	if metricsManager == nil {
		return []*MetricsInfo{}
	}

	if channelID != "" {
		m := metricsManager.GetChannelMetrics(channelID)
		return []*MetricsInfo{{
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
		}}
	}

	allMetrics := metricsManager.GetAllChannelMetrics()
	result := make([]*MetricsInfo, 0, len(allMetrics))
	for _, m := range allMetrics {
		result = append(result, &MetricsInfo{
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

	return result
}

// GetGlobalStats returns global statistics
func GetGlobalStats() GlobalStatsInfo {
	proxyMutex.RLock()
	defer proxyMutex.RUnlock()

	if globalProxyServer == nil {
		return GlobalStatsInfo{}
	}

	metricsManager := globalProxyServer.GetMetricsManager()
	if metricsManager == nil {
		return GlobalStatsInfo{}
	}

	stats := metricsManager.GetGlobalStats()
	getInt64 := func(v interface{}) int64 {
		switch t := v.(type) {
		case int64:
			return t
		case int:
			return int64(t)
		case float64:
			return int64(t)
		default:
			return 0
		}
	}
	getFloat64 := func(v interface{}) float64 {
		switch t := v.(type) {
		case float64:
			return t
		case int64:
			return float64(t)
		case int:
			return float64(t)
		default:
			return 0
		}
	}
	getInt := func(v interface{}) int {
		switch t := v.(type) {
		case int:
			return t
		case int64:
			return int(t)
		case float64:
			return int(t)
		default:
			return 0
		}
	}
	return GlobalStatsInfo{
		TotalRequests: getInt64(stats["totalRequests"]),
		SuccessCount:  getInt64(stats["successCount"]),
		FailureCount:  getInt64(stats["failureCount"]),
		SuccessRate:   getFloat64(stats["successRate"]),
		ChannelCount:  getInt(stats["channelCount"]),
	}
}

// ResetScheduler resets the circuit breaker for a channel
func ResetScheduler(channelID string) error {
	proxyMutex.Lock()
	defer proxyMutex.Unlock()

	if globalProxyServer == nil {
		return fmt.Errorf("proxy server not initialized")
	}

	globalProxyServer.GetScheduler().ResetCircuit(channelID)
	return nil
}

// GetRequestHistory returns request history records
func GetRequestHistory(channelID string, limit, offset int, statusFilter string) ([]*RequestHistoryRecord, int64) {
	proxyMutex.RLock()
	defer proxyMutex.RUnlock()

	if globalProxyServer == nil {
		return []*RequestHistoryRecord{}, 0
	}

	historyMgr := globalProxyServer.GetHistoryManager()
	if historyMgr == nil {
		return []*RequestHistoryRecord{}, 0
	}

	records, totalCount := historyMgr.GetHistory(channelID, limit, offset, statusFilter)
	result := make([]*RequestHistoryRecord, len(records))
	for i, r := range records {
		result[i] = &RequestHistoryRecord{
			ID:           r.ID,
			Timestamp:    r.Timestamp.Format(time.RFC3339),
			ChannelID:    r.ChannelID,
			ChannelType:  r.ChannelType,
			Model:        r.Model,
			Success:      r.Success,
			LatencyMs:    r.LatencyMs,
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
			ErrorMsg:     r.ErrorMsg,
			ErrorDetails: r.ErrorDetails,
		}
	}

	return result, totalCount
}

// RecordHistoryRequest records a request to history (called by handlers)
func RecordHistoryRequest(channelID, channelType, model string, success bool, latencyMs, inputTokens, outputTokens int64, errorMsg string, errorDetails string) {
	proxyMutex.RLock()
	defer proxyMutex.RUnlock()

	if globalProxyServer == nil {
		return
	}

	historyMgr := globalProxyServer.GetHistoryManager()
	if historyMgr == nil {
		return
	}

	historyMgr.RecordRequest(channelID, channelType, model, success, latencyMs, inputTokens, outputTokens, errorMsg, errorDetails)
}

// ClearRequestHistory clears all request history records.
func ClearRequestHistory() error {
	proxyMutex.Lock()
	defer proxyMutex.Unlock()

	if globalProxyServer == nil {
		return nil
	}

	historyMgr := globalProxyServer.GetHistoryManager()
	if historyMgr == nil {
		return nil
	}

	historyMgr.Clear()
	return nil
}
