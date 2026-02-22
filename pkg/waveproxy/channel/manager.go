// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Package channel provides channel management for the proxy service.
package channel

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sanshao85/tideterm/pkg/waveproxy/config"
)

// ChannelType represents the type of API channels
type ChannelType string

const (
	ChannelTypeMessages  ChannelType = "messages"
	ChannelTypeResponses ChannelType = "responses"
	ChannelTypeGemini    ChannelType = "gemini"
)

// ChannelInfo contains channel information for scheduling
type ChannelInfo struct {
	Index    int
	ID       string
	Name     string
	Priority int
	Status   string
}

// Manager manages all channels
type Manager struct {
	mu sync.RWMutex

	// Channels by type
	channels         []config.Channel
	responseChannels []config.Channel
	geminiChannels   []config.Channel

	// Failed keys cache
	failedKeys      map[string]*FailedKey
	keyRecoveryTime time.Duration
	maxFailureCount int
}

// FailedKey tracks failed API keys
type FailedKey struct {
	Timestamp    time.Time
	FailureCount int
}

// NewManager creates a new channel manager
func NewManager() *Manager {
	m := &Manager{
		failedKeys:      make(map[string]*FailedKey),
		keyRecoveryTime: 5 * time.Minute,
		maxFailureCount: 3,
	}

	// Start cleanup goroutine
	go m.cleanupExpiredFailures()

	return m
}

// LoadChannels loads channels from configuration
func (m *Manager) LoadChannels(cfg *config.Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("[ChannelManager] Loading channels from config: %d messages, %d responses, %d gemini",
		len(cfg.Channels), len(cfg.ResponseChannels), len(cfg.GeminiChannels))

	m.channels = make([]config.Channel, len(cfg.Channels))
	copy(m.channels, cfg.Channels)

	m.responseChannels = make([]config.Channel, len(cfg.ResponseChannels))
	copy(m.responseChannels, cfg.ResponseChannels)

	m.geminiChannels = make([]config.Channel, len(cfg.GeminiChannels))
	copy(m.geminiChannels, cfg.GeminiChannels)

	log.Printf("[ChannelManager] Loaded: %d messages, %d responses, %d gemini",
		len(m.channels), len(m.responseChannels), len(m.geminiChannels))
}

// Count returns the total number of channels
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.channels) + len(m.responseChannels) + len(m.geminiChannels)
}

func normalizedServiceType(serviceType string, defaultType string) string {
	normalized := strings.ToLower(strings.TrimSpace(serviceType))
	if normalized == "" {
		return defaultType
	}
	return normalized
}

func filterChannelsByServiceType(channels []config.Channel, want string, defaultType string) []config.Channel {
	if len(channels) == 0 {
		return nil
	}
	want = strings.ToLower(strings.TrimSpace(want))
	if want == "" {
		return nil
	}

	filtered := make([]config.Channel, 0, len(channels))
	for _, ch := range channels {
		serviceType := normalizedServiceType(ch.ServiceType, defaultType)
		if serviceType == want {
			filtered = append(filtered, ch)
		}
	}
	return filtered
}

func (m *Manager) channelsForTypeLocked(channelType ChannelType) []config.Channel {
	switch channelType {
	case ChannelTypeMessages:
		// Messages endpoint is Claude-compatible. Prefer channels from cfg.Channels
		// (default serviceType=claude), but allow explicitly configured Claude channels
		// from responseChannels as a fallback.
		if claudeChannels := filterChannelsByServiceType(m.channels, "claude", "claude"); len(claudeChannels) > 0 {
			return claudeChannels
		}
		if claudeChannels := filterChannelsByServiceType(m.responseChannels, "claude", "openai"); len(claudeChannels) > 0 {
			return claudeChannels
		}
		return nil
	case ChannelTypeResponses:
		// Responses endpoint is OpenAI-compatible (Codex, OpenAI SDKs).
		// Prefer OpenAI channels first (responseChannels, then channels). Only if none
		// exist do we fall back to Claude channels (protocol conversion).
		if openAIChannels := filterChannelsByServiceType(m.responseChannels, "openai", "openai"); len(openAIChannels) > 0 {
			return openAIChannels
		}
		if openAIChannels := filterChannelsByServiceType(m.channels, "openai", "claude"); len(openAIChannels) > 0 {
			return openAIChannels
		}
		if claudeChannels := filterChannelsByServiceType(m.responseChannels, "claude", "openai"); len(claudeChannels) > 0 {
			return claudeChannels
		}
		if claudeChannels := filterChannelsByServiceType(m.channels, "claude", "claude"); len(claudeChannels) > 0 {
			return claudeChannels
		}
		return nil
	case ChannelTypeGemini:
		if geminiChannels := filterChannelsByServiceType(m.geminiChannels, "gemini", "gemini"); len(geminiChannels) > 0 {
			return geminiChannels
		}
		return nil
	default:
		return nil
	}
}

// GetChannels returns channels by type
func (m *Manager) GetChannels(channelType ChannelType) []config.Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channels := m.channelsForTypeLocked(channelType)
	if len(channels) == 0 {
		return nil
	}

	// Return a copy
	result := make([]config.Channel, len(channels))
	for i := range channels {
		result[i] = *channels[i].Clone()
	}
	return result
}

// GetChannel returns a specific channel by index
func (m *Manager) GetChannel(channelType ChannelType, index int) *config.Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channels := m.channelsForTypeLocked(channelType)

	if index < 0 || index >= len(channels) {
		return nil
	}

	return channels[index].Clone()
}

// GetActiveChannels returns active channels sorted by priority
func (m *Manager) GetActiveChannels(channelType ChannelType) []ChannelInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	log.Printf("[ChannelManager] GetActiveChannels called: type=%s, messages=%d, responses=%d, gemini=%d",
		channelType, len(m.channels), len(m.responseChannels), len(m.geminiChannels))

	channels := m.channelsForTypeLocked(channelType)
	if len(channels) == 0 {
		log.Printf("[ChannelManager] Selected 0 channels for type %s", channelType)
		return nil
	}

	log.Printf("[ChannelManager] Selected %d channels for type %s", len(channels), channelType)

	var active []ChannelInfo
	for i, ch := range channels {
		status := ch.Status
		if status == "" {
			status = "active"
		}
		if status == "disabled" {
			continue
		}

		priority := ch.Priority
		if priority == 0 {
			priority = i
		}

		active = append(active, ChannelInfo{
			Index:    i,
			ID:       ch.ID,
			Name:     ch.Name,
			Priority: priority,
			Status:   status,
		})
	}

	// Sort by priority
	sort.Slice(active, func(i, j int) bool {
		return active[i].Priority < active[j].Priority
	})

	return active
}

// AddChannel adds a new channel
func (m *Manager) AddChannel(channelType ChannelType, ch config.Channel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate ID if not set
	if ch.ID == "" {
		ch.ID = generateID()
	}

	switch channelType {
	case ChannelTypeMessages:
		m.channels = append(m.channels, ch)
	case ChannelTypeResponses:
		m.responseChannels = append(m.responseChannels, ch)
	case ChannelTypeGemini:
		m.geminiChannels = append(m.geminiChannels, ch)
	default:
		return fmt.Errorf("unknown channel type: %s", channelType)
	}

	return nil
}

// UpdateChannel updates an existing channel
func (m *Manager) UpdateChannel(channelType ChannelType, index int, ch config.Channel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var channels *[]config.Channel
	switch channelType {
	case ChannelTypeMessages:
		channels = &m.channels
	case ChannelTypeResponses:
		channels = &m.responseChannels
	case ChannelTypeGemini:
		channels = &m.geminiChannels
	default:
		return fmt.Errorf("unknown channel type: %s", channelType)
	}

	if index < 0 || index >= len(*channels) {
		return fmt.Errorf("channel index out of range: %d", index)
	}

	(*channels)[index] = ch
	return nil
}

// DeleteChannel removes a channel
func (m *Manager) DeleteChannel(channelType ChannelType, index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var channels *[]config.Channel
	switch channelType {
	case ChannelTypeMessages:
		channels = &m.channels
	case ChannelTypeResponses:
		channels = &m.responseChannels
	case ChannelTypeGemini:
		channels = &m.geminiChannels
	default:
		return fmt.Errorf("unknown channel type: %s", channelType)
	}

	if index < 0 || index >= len(*channels) {
		return fmt.Errorf("channel index out of range: %d", index)
	}

	*channels = append((*channels)[:index], (*channels)[index+1:]...)
	return nil
}

// GetNextAPIKey returns the next available API key for a channel
func (m *Manager) GetNextAPIKey(ch *config.Channel, failedKeys map[string]bool) (string, error) {
	enabledKeys := ch.EnabledAPIKeys()
	if len(enabledKeys) == 0 {
		return "", fmt.Errorf("no API keys available for channel %s", ch.Name)
	}

	// Single key - return it
	if len(enabledKeys) == 1 {
		return enabledKeys[0], nil
	}

	// Find first available key
	for _, key := range enabledKeys {
		if failedKeys[key] {
			continue
		}
		if m.isKeyFailed(key) {
			continue
		}
		return key, nil
	}

	// All keys failed - return the oldest failed one for retry
	m.mu.RLock()
	var oldestKey string
	var oldestTime time.Time
	for _, key := range enabledKeys {
		if failedKeys[key] {
			continue
		}
		if failure, exists := m.failedKeys[key]; exists {
			if oldestKey == "" || failure.Timestamp.Before(oldestTime) {
				oldestKey = key
				oldestTime = failure.Timestamp
			}
		}
	}
	m.mu.RUnlock()

	if oldestKey != "" {
		return oldestKey, nil
	}

	return "", fmt.Errorf("all API keys failed for channel %s", ch.Name)
}

// MarkKeyFailed marks an API key as failed
func (m *Manager) MarkKeyFailed(apiKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if failure, exists := m.failedKeys[apiKey]; exists {
		failure.FailureCount++
		failure.Timestamp = time.Now()
	} else {
		m.failedKeys[apiKey] = &FailedKey{
			Timestamp:    time.Now(),
			FailureCount: 1,
		}
	}
}

// isKeyFailed checks if a key is in failed state
func (m *Manager) isKeyFailed(apiKey string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	failure, exists := m.failedKeys[apiKey]
	if !exists {
		return false
	}

	recoveryTime := m.keyRecoveryTime
	if failure.FailureCount > m.maxFailureCount {
		recoveryTime = m.keyRecoveryTime * 2
	}

	return time.Since(failure.Timestamp) < recoveryTime
}

// cleanupExpiredFailures periodically cleans up expired failure records
func (m *Manager) cleanupExpiredFailures() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for key, failure := range m.failedKeys {
			recoveryTime := m.keyRecoveryTime
			if failure.FailureCount > m.maxFailureCount {
				recoveryTime = m.keyRecoveryTime * 2
			}
			if now.Sub(failure.Timestamp) > recoveryTime {
				delete(m.failedKeys, key)
			}
		}
		m.mu.Unlock()
	}
}

// generateID generates a unique ID
func generateID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("ch_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("ch_%s", hex.EncodeToString(bytes))
}
