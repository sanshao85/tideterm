// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Package scheduler provides intelligent channel scheduling with priority,
// affinity, circuit breaking, and automatic failover.
package scheduler

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/sanshao85/tideterm/pkg/waveproxy/channel"
	"github.com/sanshao85/tideterm/pkg/waveproxy/config"
	"github.com/sanshao85/tideterm/pkg/waveproxy/metrics"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation
	CircuitOpen                         // Failures exceeded threshold
	CircuitHalfOpen                     // Testing recovery
)

// CircuitBreaker manages circuit breaker state for a channel
type CircuitBreaker struct {
	State         CircuitState
	FailureCount  int
	LastFailure   time.Time
	LastSuccess   time.Time
	OpenedAt      time.Time
	HalfOpenTrips int
}

type keyAffinityEntry struct {
	apiKey    string
	expiresAt time.Time
}

// SchedulerConfig contains scheduler configuration
type SchedulerConfig struct {
	FailureThreshold    int           // Failures before opening circuit
	SuccessThreshold    int           // Successes to close half-open circuit
	OpenDuration        time.Duration // Time before trying half-open
	HalfOpenMaxAttempts int           // Max attempts in half-open state
}

// DefaultSchedulerConfig returns default scheduler configuration
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		OpenDuration:        30 * time.Second,
		HalfOpenMaxAttempts: 3,
	}
}

// Scheduler manages channel selection and circuit breaking
type Scheduler struct {
	mu sync.RWMutex

	channels *channel.Manager
	metrics  *metrics.Manager
	config   SchedulerConfig

	// Circuit breakers by channel ID
	breakers map[string]*CircuitBreaker

	// User affinity: userID -> channelID
	affinity map[string]string

	// Key affinity: userID|channelID -> apiKey
	keyAffinity map[string]keyAffinityEntry
}

// NewScheduler creates a new scheduler
func NewScheduler(channels *channel.Manager, metrics *metrics.Manager) *Scheduler {
	return &Scheduler{
		channels:    channels,
		metrics:     metrics,
		config:      DefaultSchedulerConfig(),
		breakers:    make(map[string]*CircuitBreaker),
		affinity:    make(map[string]string),
		keyAffinity: make(map[string]keyAffinityEntry),
	}
}

// SelectChannel selects the best channel for a request
func (s *Scheduler) SelectChannel(channelType channel.ChannelType, userID string, excludeChannels map[string]bool) (*config.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get active channels sorted by priority
	activeChannels := s.channels.GetActiveChannels(channelType)
	log.Printf("[Scheduler] SelectChannel: type=%s, activeChannels=%d, excludeChannels=%v", channelType, len(activeChannels), excludeChannels)
	for i, ch := range activeChannels {
		log.Printf("[Scheduler] ActiveChannel[%d]: id=%s, name=%s, status=%s, priority=%d", i, ch.ID, ch.Name, ch.Status, ch.Priority)
	}
	if len(activeChannels) == 0 {
		return nil, fmt.Errorf("no active channels available")
	}

	// Check user affinity first
	if userID != "" {
		if affinityChannelID, ok := s.affinity[userID]; ok {
			for _, info := range activeChannels {
				if info.ID == affinityChannelID && !excludeChannels[info.ID] {
					if s.isChannelAvailable(info.ID) {
						ch := s.channels.GetChannel(channelType, info.Index)
						if ch != nil {
							log.Printf("[Scheduler-Affinity] Using affinity channel %s for user %s", info.Name, userID)
							return ch, nil
						}
					}
				}
			}
		}
	}

	// Find promotion channels first
	for _, info := range activeChannels {
		if excludeChannels[info.ID] {
			continue
		}

		ch := s.channels.GetChannel(channelType, info.Index)
		if ch == nil {
			continue
		}

		if ch.IsInPromotion() && s.isChannelAvailable(info.ID) {
			log.Printf("[Scheduler-Promotion] Using promotion channel %s", info.Name)
			s.setAffinity(userID, info.ID)
			return ch, nil
		}
	}

	// Select by priority
	for _, info := range activeChannels {
		if excludeChannels[info.ID] {
			continue
		}

		if !s.isChannelAvailable(info.ID) {
			continue
		}

		ch := s.channels.GetChannel(channelType, info.Index)
		if ch != nil {
			log.Printf("[Scheduler-Channel] Selected channel %s (priority %d)", info.Name, info.Priority)
			s.setAffinity(userID, info.ID)
			return ch, nil
		}
	}

	// No available channels - try to find one in half-open state for recovery
	for _, info := range activeChannels {
		if excludeChannels[info.ID] {
			continue
		}

		breaker := s.getBreaker(info.ID)
		if breaker.State == CircuitHalfOpen {
			ch := s.channels.GetChannel(channelType, info.Index)
			if ch != nil {
				log.Printf("[Scheduler-Recovery] Trying half-open channel %s", info.Name)
				return ch, nil
			}
		}
	}

	return nil, fmt.Errorf("all channels are unavailable or circuit broken")
}

// RecordSuccess records a successful request for a channel
func (s *Scheduler) RecordSuccess(channelID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	breaker := s.getBreaker(channelID)
	breaker.LastSuccess = time.Now()

	switch breaker.State {
	case CircuitHalfOpen:
		breaker.FailureCount = 0
		breaker.HalfOpenTrips++
		if breaker.HalfOpenTrips >= s.config.SuccessThreshold {
			breaker.State = CircuitClosed
			breaker.HalfOpenTrips = 0
			log.Printf("[Scheduler-CircuitBreaker] Channel %s circuit closed after recovery", channelID)
		}
	case CircuitClosed:
		breaker.FailureCount = 0
	}
}

// RecordFailure records a failed request for a channel
func (s *Scheduler) RecordFailure(channelID string, isRetryable bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	breaker := s.getBreaker(channelID)
	breaker.LastFailure = time.Now()
	if !isRetryable {
		// Do not trip circuit breakers on permanent/non-retryable failures (e.g. 400/401/403),
		// which are often request- or credential-specific rather than channel health issues.
		return
	}

	breaker.FailureCount++

	switch breaker.State {
	case CircuitClosed:
		if breaker.FailureCount >= s.config.FailureThreshold {
			breaker.State = CircuitOpen
			breaker.OpenedAt = time.Now()
			log.Printf("[Scheduler-CircuitBreaker] Channel %s circuit opened after %d failures", channelID, breaker.FailureCount)
		}
	case CircuitHalfOpen:
		// Failed during recovery - back to open
		breaker.State = CircuitOpen
		breaker.OpenedAt = time.Now()
		breaker.HalfOpenTrips = 0
		log.Printf("[Scheduler-CircuitBreaker] Channel %s circuit re-opened after half-open failure", channelID)
	}
}

// ResetCircuit manually resets the circuit breaker for a channel
func (s *Scheduler) ResetCircuit(channelID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if breaker, exists := s.breakers[channelID]; exists {
		breaker.State = CircuitClosed
		breaker.FailureCount = 0
		breaker.HalfOpenTrips = 0
		log.Printf("[Scheduler-CircuitBreaker] Channel %s circuit manually reset", channelID)
	}
}

// GetCircuitState returns the circuit breaker state for a channel
func (s *Scheduler) GetCircuitState(channelID string) CircuitState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if breaker, exists := s.breakers[channelID]; exists {
		return breaker.State
	}
	return CircuitClosed
}

// GetSchedulerStats returns scheduler statistics
func (s *Scheduler) GetSchedulerStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]interface{})

	breakerStats := make(map[string]map[string]interface{})
	for channelID, breaker := range s.breakers {
		breakerStats[channelID] = map[string]interface{}{
			"state":         stateToString(breaker.State),
			"failureCount":  breaker.FailureCount,
			"lastFailure":   breaker.LastFailure,
			"lastSuccess":   breaker.LastSuccess,
			"halfOpenTrips": breaker.HalfOpenTrips,
		}
	}
	stats["circuitBreakers"] = breakerStats
	stats["affinityCount"] = len(s.affinity)
	stats["config"] = s.config

	return stats
}

// ClearAffinity clears user affinity mapping
func (s *Scheduler) ClearAffinity(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.affinity, userID)
}

// GetKeyAffinity returns the affinity API key for a user + channel if it is still valid.
func (s *Scheduler) GetKeyAffinity(userID string, channelID string) (string, bool) {
	if userID == "" || channelID == "" {
		return "", false
	}
	affinityKey := makeKeyAffinityKey(userID, channelID)

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.keyAffinity[affinityKey]
	if !ok {
		return "", false
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		delete(s.keyAffinity, affinityKey)
		return "", false
	}
	return entry.apiKey, true
}

// SetKeyAffinity sets the affinity API key for a user + channel with optional TTL.
func (s *Scheduler) SetKeyAffinity(userID string, channelID string, apiKey string, ttl time.Duration) {
	if userID == "" || channelID == "" || apiKey == "" {
		return
	}
	affinityKey := makeKeyAffinityKey(userID, channelID)

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.keyAffinity[affinityKey] = keyAffinityEntry{apiKey: apiKey, expiresAt: expiresAt}
}

// ClearKeyAffinity removes the affinity API key for a user + channel.
func (s *Scheduler) ClearKeyAffinity(userID string, channelID string) {
	if userID == "" || channelID == "" {
		return
	}
	affinityKey := makeKeyAffinityKey(userID, channelID)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.keyAffinity, affinityKey)
}

// isChannelAvailable checks if a channel is available based on circuit breaker state
func (s *Scheduler) isChannelAvailable(channelID string) bool {
	breaker := s.getBreaker(channelID)

	switch breaker.State {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if we should transition to half-open
		if time.Since(breaker.OpenedAt) >= s.config.OpenDuration {
			breaker.State = CircuitHalfOpen
			breaker.HalfOpenTrips = 0
			log.Printf("[Scheduler-CircuitBreaker] Channel %s transitioning to half-open", channelID)
			return true
		}
		return false
	case CircuitHalfOpen:
		// Allow limited requests in half-open state
		return breaker.HalfOpenTrips < s.config.HalfOpenMaxAttempts
	}
	return false
}

// getBreaker gets or creates a circuit breaker for a channel
func (s *Scheduler) getBreaker(channelID string) *CircuitBreaker {
	if breaker, exists := s.breakers[channelID]; exists {
		return breaker
	}

	breaker := &CircuitBreaker{
		State: CircuitClosed,
	}
	s.breakers[channelID] = breaker
	return breaker
}

// setAffinity sets user affinity to a channel
func (s *Scheduler) setAffinity(userID string, channelID string) {
	if userID != "" {
		s.affinity[userID] = channelID
	}
}

func makeKeyAffinityKey(userID string, channelID string) string {
	return userID + "|" + channelID
}

// stateToString converts circuit state to string
func stateToString(state CircuitState) string {
	switch state {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}
