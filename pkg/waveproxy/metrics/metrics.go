// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Package metrics provides metrics collection and management for channels.
package metrics

import (
	"sync"
	"time"
)

// ChannelMetrics contains metrics for a single channel
type ChannelMetrics struct {
	ChannelID           string     `json:"channelId"`
	RequestCount        int64      `json:"requestCount"`
	SuccessCount        int64      `json:"successCount"`
	FailureCount        int64      `json:"failureCount"`
	SuccessRate         float64    `json:"successRate"`
	ConsecutiveFailures int64      `json:"consecutiveFailures"`
	CircuitBroken       bool       `json:"circuitBroken"`
	LastSuccessAt       *time.Time `json:"lastSuccessAt,omitempty"`
	LastFailureAt       *time.Time `json:"lastFailureAt,omitempty"`
	InputTokens         int64      `json:"inputTokens"`
	OutputTokens        int64      `json:"outputTokens"`
	CacheReadTokens     int64      `json:"cacheReadTokens"`
	CacheCreateTokens   int64      `json:"cacheCreateTokens"`
	CacheHitRate        float64    `json:"cacheHitRate"`
	AvgLatencyMs        float64    `json:"avgLatencyMs"`
}

// RequestRecord represents a single request for sliding window calculation
type RequestRecord struct {
	Timestamp   time.Time
	Success     bool
	LatencyMs   int64
	InputTokens int64
	OutputTokens int64
	CacheRead   int64
	CacheCreate int64
}

// Manager manages metrics collection and calculation
type Manager struct {
	mu sync.RWMutex

	// Sliding window configuration
	windowSize       int     // Number of records to keep
	failureThreshold float64 // Failure rate threshold for alerts

	// Metrics data by channel ID
	records map[string][]RequestRecord
	metrics map[string]*ChannelMetrics

	// Global statistics
	globalRequests int64
	globalSuccess  int64
	globalFailures int64

	stopCh chan struct{}
}

// NewManager creates a new metrics manager
func NewManager(windowSize int, failureThreshold float64) *Manager {
	if windowSize < 3 {
		windowSize = 10
	}
	if failureThreshold <= 0 || failureThreshold > 1 {
		failureThreshold = 0.5
	}

	m := &Manager{
		windowSize:       windowSize,
		failureThreshold: failureThreshold,
		records:          make(map[string][]RequestRecord),
		metrics:          make(map[string]*ChannelMetrics),
		stopCh:           make(chan struct{}),
	}

	// Start cleanup goroutine
	go m.periodicCleanup()

	return m
}

// Stop stops the metrics manager
func (m *Manager) Stop() {
	close(m.stopCh)
}

// RecordRequest records a request result
func (m *Manager) RecordRequest(channelID string, success bool, latencyMs int64, inputTokens, outputTokens, cacheRead, cacheCreate int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record := RequestRecord{
		Timestamp:    time.Now(),
		Success:      success,
		LatencyMs:    latencyMs,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CacheRead:    cacheRead,
		CacheCreate:  cacheCreate,
	}

	// Add record to sliding window
	records := m.records[channelID]
	records = append(records, record)
	if len(records) > m.windowSize {
		records = records[len(records)-m.windowSize:]
	}
	m.records[channelID] = records

	// Update metrics
	metrics := m.getOrCreateMetrics(channelID)
	metrics.RequestCount++
	metrics.InputTokens += inputTokens
	metrics.OutputTokens += outputTokens
	metrics.CacheReadTokens += cacheRead
	metrics.CacheCreateTokens += cacheCreate

	if success {
		metrics.SuccessCount++
		metrics.ConsecutiveFailures = 0
		now := time.Now()
		metrics.LastSuccessAt = &now
		m.globalSuccess++
	} else {
		metrics.FailureCount++
		metrics.ConsecutiveFailures++
		now := time.Now()
		metrics.LastFailureAt = &now
		m.globalFailures++
	}
	m.globalRequests++

	// Recalculate sliding window metrics
	m.recalculateMetrics(channelID)
}

// GetChannelMetrics returns metrics for a specific channel
func (m *Manager) GetChannelMetrics(channelID string) *ChannelMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if metrics, exists := m.metrics[channelID]; exists {
		// Return a copy
		copy := *metrics
		return &copy
	}

	return &ChannelMetrics{ChannelID: channelID}
}

// GetAllChannelMetrics returns metrics for all channels
func (m *Manager) GetAllChannelMetrics() map[string]*ChannelMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*ChannelMetrics)
	for id, metrics := range m.metrics {
		copy := *metrics
		result[id] = &copy
	}
	return result
}

// GetGlobalStats returns global statistics
func (m *Manager) GetGlobalStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var successRate float64
	if m.globalRequests > 0 {
		successRate = float64(m.globalSuccess) / float64(m.globalRequests)
	}

	return map[string]interface{}{
		"totalRequests": m.globalRequests,
		"successCount":  m.globalSuccess,
		"failureCount":  m.globalFailures,
		"successRate":   successRate,
		"channelCount":  len(m.metrics),
	}
}

// IsFailureRateHigh checks if a channel's failure rate exceeds threshold
func (m *Manager) IsFailureRateHigh(channelID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := m.records[channelID]
	if len(records) < 3 {
		return false
	}

	failures := 0
	for _, r := range records {
		if !r.Success {
			failures++
		}
	}

	failureRate := float64(failures) / float64(len(records))
	return failureRate >= m.failureThreshold
}

// SetCircuitBroken updates the circuit broken status
func (m *Manager) SetCircuitBroken(channelID string, broken bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics := m.getOrCreateMetrics(channelID)
	metrics.CircuitBroken = broken
}

// ResetChannelMetrics resets metrics for a channel
func (m *Manager) ResetChannelMetrics(channelID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.records, channelID)
	delete(m.metrics, channelID)
}

// getOrCreateMetrics gets or creates metrics for a channel
func (m *Manager) getOrCreateMetrics(channelID string) *ChannelMetrics {
	if metrics, exists := m.metrics[channelID]; exists {
		return metrics
	}

	metrics := &ChannelMetrics{ChannelID: channelID}
	m.metrics[channelID] = metrics
	return metrics
}

// recalculateMetrics recalculates sliding window metrics
func (m *Manager) recalculateMetrics(channelID string) {
	records := m.records[channelID]
	if len(records) == 0 {
		return
	}

	metrics := m.getOrCreateMetrics(channelID)

	// Calculate success rate from sliding window
	successCount := 0
	var totalLatency int64
	for _, r := range records {
		if r.Success {
			successCount++
		}
		totalLatency += r.LatencyMs
	}

	metrics.SuccessRate = float64(successCount) / float64(len(records))
	metrics.AvgLatencyMs = float64(totalLatency) / float64(len(records))

	// Calculate cache hit rate
	totalInput := metrics.InputTokens
	if totalInput > 0 {
		metrics.CacheHitRate = float64(metrics.CacheReadTokens) / float64(totalInput)
	}
}

// periodicCleanup removes old records periodically
func (m *Manager) periodicCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.stopCh:
			return
		}
	}
}

// cleanup removes records older than 24 hours
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-24 * time.Hour)

	for channelID, records := range m.records {
		var filtered []RequestRecord
		for _, r := range records {
			if r.Timestamp.After(cutoff) {
				filtered = append(filtered, r)
			}
		}
		if len(filtered) > 0 {
			m.records[channelID] = filtered
		} else {
			delete(m.records, channelID)
		}
	}
}
