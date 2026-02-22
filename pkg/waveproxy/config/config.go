// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Package config provides configuration management for the proxy service.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Config represents the main proxy configuration
type Config struct {
	// Server settings
	Port      int    `json:"port"`
	AccessKey string `json:"accessKey"`

	// Metrics settings
	MetricsWindowSize       int     `json:"metricsWindowSize"`
	MetricsFailureThreshold float64 `json:"metricsFailureThreshold"`

	// Session settings
	SessionMaxAge      time.Duration `json:"sessionMaxAge"`
	SessionMaxMessages int           `json:"sessionMaxMessages"`
	SessionMaxTokens   int           `json:"sessionMaxTokens"`

	// Feature flags
	FuzzyModeEnabled bool `json:"fuzzyModeEnabled"`
	EnableWebUI      bool `json:"enableWebUI"`

	// Channels configuration
	Channels         []Channel `json:"channels"`
	ResponseChannels []Channel `json:"responseChannels"`
	GeminiChannels   []Channel `json:"geminiChannels"`
}

// AuthType constants for channel authentication
const (
	AuthTypeAPIKey = "x-api-key" // Default: only x-api-key header
	AuthTypeBearer = "bearer"    // Only Authorization: Bearer header
	AuthTypeBoth   = "both"      // Both x-api-key and Authorization: Bearer headers
	// Gemini-compatible API key header
	AuthTypeGoogAPIKey = "x-goog-api-key"
)

// Channel represents an upstream AI service channel
type Channel struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	ServiceType        string            `json:"serviceType"` // claude, openai, gemini
	BaseURL            string            `json:"baseUrl"`
	BaseURLs           []string          `json:"baseUrls,omitempty"`
	APIKeys            []APIKey          `json:"apiKeys"`
	AuthType           string            `json:"authType,omitempty"` // x-api-key, bearer, both
	Priority           int               `json:"priority"`
	Status             string            `json:"status"` // active, suspended, disabled
	PromotionUntil     *time.Time        `json:"promotionUntil,omitempty"`
	ModelMapping       map[string]string `json:"modelMapping,omitempty"`
	LowQuality         bool              `json:"lowQuality,omitempty"`
	InsecureSkipVerify bool              `json:"insecureSkipVerify,omitempty"`
	Description        string            `json:"description,omitempty"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Port:                    3000,
		AccessKey:               "",
		MetricsWindowSize:       10,
		MetricsFailureThreshold: 0.5,
		SessionMaxAge:           24 * time.Hour,
		SessionMaxMessages:      100,
		SessionMaxTokens:        100000,
		FuzzyModeEnabled:        false,
		EnableWebUI:             true,
		Channels:                []Channel{},
		ResponseChannels:        []Channel{},
		GeminiChannels:          []Channel{},
	}
}

// LoadConfig loads a proxy configuration from a JSON file.
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	if c.MetricsWindowSize < 3 {
		c.MetricsWindowSize = 3
	}
	if c.MetricsFailureThreshold <= 0 || c.MetricsFailureThreshold > 1 {
		c.MetricsFailureThreshold = 0.5
	}
	return nil
}

// Clone creates a deep copy of the configuration
func (c *Config) Clone() *Config {
	clone := *c
	clone.Channels = make([]Channel, len(c.Channels))
	copy(clone.Channels, c.Channels)
	clone.ResponseChannels = make([]Channel, len(c.ResponseChannels))
	copy(clone.ResponseChannels, c.ResponseChannels)
	clone.GeminiChannels = make([]Channel, len(c.GeminiChannels))
	copy(clone.GeminiChannels, c.GeminiChannels)
	return &clone
}

// Clone creates a deep copy of a channel
func (ch *Channel) Clone() *Channel {
	clone := *ch
	if ch.BaseURLs != nil {
		clone.BaseURLs = make([]string, len(ch.BaseURLs))
		copy(clone.BaseURLs, ch.BaseURLs)
	}
	if ch.APIKeys != nil {
		clone.APIKeys = make([]APIKey, len(ch.APIKeys))
		copy(clone.APIKeys, ch.APIKeys)
	}
	if ch.ModelMapping != nil {
		clone.ModelMapping = make(map[string]string)
		for k, v := range ch.ModelMapping {
			clone.ModelMapping[k] = v
		}
	}
	return &clone
}

// GetAllBaseURLs returns all base URLs for the channel
func (ch *Channel) GetAllBaseURLs() []string {
	if len(ch.BaseURLs) > 0 {
		return ch.BaseURLs
	}
	if ch.BaseURL != "" {
		return []string{ch.BaseURL}
	}
	return []string{}
}

// IsInPromotion checks if the channel is in promotion period
func (ch *Channel) IsInPromotion() bool {
	if ch.PromotionUntil == nil {
		return false
	}
	return time.Now().Before(*ch.PromotionUntil)
}

// Manager handles configuration loading, saving, and hot-reloading
type Manager struct {
	mu       sync.RWMutex
	config   *Config
	filePath string
	watcher  *fsnotify.Watcher
	stopCh   chan struct{}

	// Callbacks for config changes
	onChange func(*Config)
}

// NewManager creates a new configuration manager
func NewManager(filePath string) (*Manager, error) {
	m := &Manager{
		filePath: filePath,
		stopCh:   make(chan struct{}),
	}

	// Load initial config
	if err := m.load(); err != nil {
		// If file doesn't exist, use defaults
		m.config = DefaultConfig()
	}

	// Setup file watcher for hot-reload
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}
	m.watcher = watcher

	// Watch config file
	if filePath != "" {
		go m.watchConfig()
	}

	return m, nil
}

// Get returns the current configuration (thread-safe copy)
func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Clone()
}

// Update updates the configuration and saves to file
func (m *Manager) Update(cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	m.config = cfg.Clone()
	m.mu.Unlock()

	if m.filePath != "" {
		if err := m.save(); err != nil {
			return err
		}
	}

	if m.onChange != nil {
		m.onChange(cfg)
	}

	return nil
}

// OnChange sets a callback for configuration changes
func (m *Manager) OnChange(fn func(*Config)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = fn
}

// Close stops the configuration manager
func (m *Manager) Close() error {
	close(m.stopCh)
	if m.watcher != nil {
		return m.watcher.Close()
	}
	return nil
}

// load reads configuration from file
func (m *Manager) load() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	m.config = &cfg
	m.mu.Unlock()

	return nil
}

// save writes configuration to file
func (m *Manager) save() error {
	m.mu.RLock()
	cfg := m.config
	m.mu.RUnlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Config can contain secrets (API keys), so keep permissions restricted.
	return os.WriteFile(m.filePath, data, 0600)
}

// watchConfig watches for configuration file changes
func (m *Manager) watchConfig() {
	if m.filePath == "" {
		return
	}

	if err := m.watcher.Add(m.filePath); err != nil {
		return
	}

	for {
		select {
		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				if err := m.load(); err == nil {
					if m.onChange != nil {
						m.onChange(m.config)
					}
				}
			}
		case <-m.watcher.Errors:
			// Ignore errors
		case <-m.stopCh:
			return
		}
	}
}
