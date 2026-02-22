// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Package session provides session management for multi-turn conversations.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Message represents a single message in a session
type Message struct {
	Role       string    `json:"role"`
	Content    string    `json:"content"`
	ResponseID string    `json:"responseId,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// Session represents a conversation session
type Session struct {
	ID         string    `json:"id"`
	Messages   []Message `json:"messages"`
	CreatedAt  time.Time `json:"createdAt"`
	LastAccess time.Time `json:"lastAccess"`
	TokenCount int       `json:"tokenCount"`
	ChannelID  string    `json:"channelId,omitempty"`
}

// Manager manages conversation sessions
type Manager struct {
	mu sync.RWMutex

	sessions    map[string]*Session
	responseMap map[string]string // responseID -> sessionID

	maxAge      time.Duration
	maxMessages int
	maxTokens   int

	stopCh chan struct{}
}

// NewManager creates a new session manager
func NewManager(maxAge time.Duration, maxMessages int, maxTokens int) *Manager {
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}
	if maxMessages <= 0 {
		maxMessages = 100
	}
	if maxTokens <= 0 {
		maxTokens = 100000
	}

	m := &Manager{
		sessions:    make(map[string]*Session),
		responseMap: make(map[string]string),
		maxAge:      maxAge,
		maxMessages: maxMessages,
		maxTokens:   maxTokens,
		stopCh:      make(chan struct{}),
	}

	// Start cleanup goroutine
	go m.periodicCleanup()

	return m
}

// Stop stops the session manager
func (m *Manager) Stop() {
	close(m.stopCh)
}

// CreateSession creates a new session
func (m *Manager) CreateSession() *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	session := &Session{
		ID:         generateSessionID(),
		Messages:   make([]Message, 0),
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
	}

	m.sessions[session.ID] = session
	return session
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(sessionID string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if session, exists := m.sessions[sessionID]; exists {
		return session
	}
	return nil
}

// GetSessionByResponseID retrieves a session by previous response ID
func (m *Manager) GetSessionByResponseID(responseID string) *Session {
	m.mu.RLock()
	sessionID, exists := m.responseMap[responseID]
	m.mu.RUnlock()

	if !exists {
		return nil
	}

	return m.GetSession(sessionID)
}

// GetOrCreateSession gets an existing session or creates a new one
func (m *Manager) GetOrCreateSession(previousResponseID string) (*Session, bool) {
	if previousResponseID != "" {
		if session := m.GetSessionByResponseID(previousResponseID); session != nil {
			m.mu.Lock()
			session.LastAccess = time.Now()
			m.mu.Unlock()
			return session, false
		}
	}

	return m.CreateSession(), true
}

// AddMessage adds a message to a session
func (m *Manager) AddMessage(sessionID string, role string, content string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}

	// Check message limit
	if len(session.Messages) >= m.maxMessages {
		// Remove oldest messages
		excess := len(session.Messages) - m.maxMessages + 1
		session.Messages = session.Messages[excess:]
	}

	responseID := generateResponseID()
	message := Message{
		Role:       role,
		Content:    content,
		ResponseID: responseID,
		Timestamp:  time.Now(),
	}

	session.Messages = append(session.Messages, message)
	session.LastAccess = time.Now()
	session.TokenCount += estimateTokens(content)

	// Map response ID to session
	m.responseMap[responseID] = sessionID

	return responseID, nil
}

// GetMessages retrieves all messages from a session
func (m *Manager) GetMessages(sessionID string) []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil
	}

	// Return a copy
	messages := make([]Message, len(session.Messages))
	copy(messages, session.Messages)
	return messages
}

// SetChannelAffinity sets the channel affinity for a session
func (m *Manager) SetChannelAffinity(sessionID string, channelID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[sessionID]; exists {
		session.ChannelID = channelID
	}
}

// GetChannelAffinity gets the channel affinity for a session
func (m *Manager) GetChannelAffinity(sessionID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if session, exists := m.sessions[sessionID]; exists {
		return session.ChannelID
	}
	return ""
}

// DeleteSession deletes a session
func (m *Manager) DeleteSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return
	}

	// Clean up response mappings
	for _, msg := range session.Messages {
		if msg.ResponseID != "" {
			delete(m.responseMap, msg.ResponseID)
		}
	}

	delete(m.sessions, sessionID)
}

// GetStats returns session manager statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalMessages := 0
	totalTokens := 0
	for _, session := range m.sessions {
		totalMessages += len(session.Messages)
		totalTokens += session.TokenCount
	}

	return map[string]interface{}{
		"sessionCount":   len(m.sessions),
		"messageCount":   totalMessages,
		"tokenCount":     totalTokens,
		"maxAge":         m.maxAge.String(),
		"maxMessages":    m.maxMessages,
		"maxTokens":      m.maxTokens,
		"responseMapped": len(m.responseMap),
	}
}

// periodicCleanup removes expired sessions
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

// cleanup removes expired sessions
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-m.maxAge)

	for sessionID, session := range m.sessions {
		if session.LastAccess.Before(cutoff) {
			// Clean up response mappings
			for _, msg := range session.Messages {
				if msg.ResponseID != "" {
					delete(m.responseMap, msg.ResponseID)
				}
			}
			delete(m.sessions, sessionID)
		}
	}
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("sess_%s", hex.EncodeToString(bytes))
}

// generateResponseID generates a unique response ID
func generateResponseID() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("resp_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("resp_%s", hex.EncodeToString(bytes))
}

// estimateTokens provides a rough token count estimate
func estimateTokens(content string) int {
	// Rough estimate: ~4 characters per token
	return len(content) / 4
}
