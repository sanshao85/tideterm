// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Package history provides request history storage and retrieval.
package history

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// RequestRecord represents a single API request in history
type RequestRecord struct {
	ID           string    `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	ChannelID    string    `json:"channelId"`
	ChannelType  string    `json:"channelType"` // messages, responses, gemini
	Model        string    `json:"model"`
	Success      bool      `json:"success"`
	LatencyMs    int64     `json:"latencyMs"`
	InputTokens  int64     `json:"inputTokens"`
	OutputTokens int64     `json:"outputTokens"`
	ErrorMsg     string    `json:"errorMsg,omitempty"`
	ErrorDetails string    `json:"errorDetails,omitempty"`
}

// Manager manages request history storage
type Manager struct {
	mu sync.RWMutex

	// Ring buffer for history records
	records    []RequestRecord
	maxRecords int
	writeIdx   int
	count      int

	retention time.Duration

	// Index for faster lookups by channel
	byChannel map[string][]int // channelID -> indices in records

	stopCh chan struct{}
}

// NewManager creates a new history manager
func NewManager(maxRecords int) *Manager {
	if maxRecords <= 0 {
		maxRecords = 1000 // Default to 1000 records
	}

	m := &Manager{
		records:    make([]RequestRecord, maxRecords),
		maxRecords: maxRecords,
		byChannel:  make(map[string][]int),
		stopCh:     make(chan struct{}),
		retention:  48 * time.Hour,
	}

	// Start periodic index rebuild
	go m.periodicMaintenance()

	return m
}

// Stop stops the history manager
func (m *Manager) Stop() {
	close(m.stopCh)
}

// RecordRequest adds a new request to history
func (m *Manager) RecordRequest(channelID, channelType, model string, success bool, latencyMs, inputTokens, outputTokens int64, errorMsg string, errorDetails string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove the soon-to-be overwritten index from the prior channel's index.
	// Without this, byChannel grows unbounded over time and can reference stale slots.
	if prev := m.records[m.writeIdx]; prev.ID != "" {
		if idxs, ok := m.byChannel[prev.ChannelID]; ok {
			for i := range idxs {
				if idxs[i] == m.writeIdx {
					idxs[i] = idxs[len(idxs)-1]
					idxs = idxs[:len(idxs)-1]
					break
				}
			}
			if len(idxs) == 0 {
				delete(m.byChannel, prev.ChannelID)
			} else {
				m.byChannel[prev.ChannelID] = idxs
			}
		}
	}

	id := uuid.New().String()
	record := RequestRecord{
		ID:           id,
		Timestamp:    time.Now(),
		ChannelID:    channelID,
		ChannelType:  channelType,
		Model:        model,
		Success:      success,
		LatencyMs:    latencyMs,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		ErrorMsg:     errorMsg,
		ErrorDetails: errorDetails,
	}

	// Store in ring buffer
	m.records[m.writeIdx] = record

	// Update channel index
	m.byChannel[channelID] = append(m.byChannel[channelID], m.writeIdx)

	// Advance write position
	m.writeIdx = (m.writeIdx + 1) % m.maxRecords
	if m.count < m.maxRecords {
		m.count++
	}

	return id
}

// GetHistory returns request history with pagination
func (m *Manager) GetHistory(channelID string, limit, offset int, statusFilter string) ([]RequestRecord, int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect all matching records (in reverse chronological order)
	var matching []RequestRecord
	cutoff := time.Time{}
	if m.retention > 0 {
		cutoff = time.Now().Add(-m.retention)
	}
	startIdx := m.writeIdx - 1
	if startIdx < 0 {
		startIdx = m.maxRecords - 1
	}

	for i := 0; i < m.count; i++ {
		idx := startIdx - i
		if idx < 0 {
			idx += m.maxRecords
		}
		record := m.records[idx]
		if record.ID == "" {
			continue
		}
		if !cutoff.IsZero() && record.Timestamp.Before(cutoff) {
			continue
		}
		if channelID == "" || record.ChannelID == channelID {
			switch statusFilter {
			case "success":
				if !record.Success {
					continue
				}
			case "error":
				if record.Success {
					continue
				}
			}
			matching = append(matching, record)
		}
	}

	totalCount := int64(len(matching))

	// Apply pagination
	if offset >= len(matching) {
		return []RequestRecord{}, totalCount
	}

	end := offset + limit
	if limit <= 0 || end > len(matching) {
		end = len(matching)
	}

	return matching[offset:end], totalCount
}

// GetRecordByID retrieves a single record by ID
func (m *Manager) GetRecordByID(id string) *RequestRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := 0; i < m.count; i++ {
		if m.records[i].ID == id {
			record := m.records[i]
			return &record
		}
	}
	return nil
}

// GetStats returns statistics about the history
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	successCount := 0
	failureCount := 0
	totalLatency := int64(0)

	for i := 0; i < m.count; i++ {
		if m.records[i].ID == "" {
			continue
		}
		if m.records[i].Success {
			successCount++
		} else {
			failureCount++
		}
		totalLatency += m.records[i].LatencyMs
	}

	avgLatency := float64(0)
	if m.count > 0 {
		avgLatency = float64(totalLatency) / float64(m.count)
	}

	return map[string]interface{}{
		"totalRecords": m.count,
		"maxRecords":   m.maxRecords,
		"successCount": successCount,
		"failureCount": failureCount,
		"avgLatencyMs": avgLatency,
	}
}

// Clear clears all history records
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.records = make([]RequestRecord, m.maxRecords)
	m.byChannel = make(map[string][]int)
	m.writeIdx = 0
	m.count = 0
}

// periodicMaintenance performs periodic cleanup and index rebuilding
func (m *Manager) periodicMaintenance() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.rebuildIndex()
		case <-m.stopCh:
			return
		}
	}
}

// rebuildIndex rebuilds the channel index
func (m *Manager) rebuildIndex() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.byChannel = make(map[string][]int)
	for i := 0; i < m.count; i++ {
		if m.records[i].ID != "" {
			m.byChannel[m.records[i].ChannelID] = append(m.byChannel[m.records[i].ChannelID], i)
		}
	}
}
