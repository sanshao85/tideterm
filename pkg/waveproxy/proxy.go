// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Package waveproxy provides AI API proxy service management.
// It supports multiple upstream providers (Claude, OpenAI, Gemini) with
// intelligent scheduling, circuit breaking, and protocol conversion.
package waveproxy

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/sanshao85/tideterm/pkg/waveproxy/channel"
	"github.com/sanshao85/tideterm/pkg/waveproxy/config"
	"github.com/sanshao85/tideterm/pkg/waveproxy/handler"
	"github.com/sanshao85/tideterm/pkg/waveproxy/history"
	"github.com/sanshao85/tideterm/pkg/waveproxy/metrics"
	"github.com/sanshao85/tideterm/pkg/waveproxy/scheduler"
	"github.com/sanshao85/tideterm/pkg/waveproxy/session"
)

// ProxyServer represents the main proxy service
type ProxyServer struct {
	mu sync.RWMutex

	config    *config.Config
	server    *http.Server
	scheduler *scheduler.Scheduler
	metrics   *metrics.Manager
	sessions  *session.Manager
	channels  *channel.Manager
	history   *history.Manager

	running   bool
	startedAt time.Time
	stopCh    chan struct{}
}

// ProxyStatus represents the current status of the proxy server
type ProxyStatus struct {
	Running      bool      `json:"running"`
	Port         int       `json:"port"`
	StartedAt    time.Time `json:"startedAt,omitempty"`
	Uptime       string    `json:"uptime,omitempty"`
	Version      string    `json:"version"`
	ChannelCount int       `json:"channelCount"`
}

// Version information
var (
	Version   = "1.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// New creates a new ProxyServer instance
func New(cfg *config.Config) (*ProxyServer, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	log.Printf("[WaveProxy-New] Creating server with config: %d channels, %d response channels, %d gemini channels",
		len(cfg.Channels), len(cfg.ResponseChannels), len(cfg.GeminiChannels))

	channelMgr := channel.NewManager()
	channelMgr.LoadChannels(cfg)

	log.Printf("[WaveProxy-New] ChannelManager created, total channels: %d", channelMgr.Count())

	metricsMgr := metrics.NewManager(cfg.MetricsWindowSize, cfg.MetricsFailureThreshold)
	sessionMgr := session.NewManager(cfg.SessionMaxAge, cfg.SessionMaxMessages, cfg.SessionMaxTokens)
	historyMgr := history.NewManager(1000) // Keep last 1000 requests
	sched := scheduler.NewScheduler(channelMgr, metricsMgr)

	return &ProxyServer{
		config:    cfg,
		channels:  channelMgr,
		metrics:   metricsMgr,
		sessions:  sessionMgr,
		history:   historyMgr,
		scheduler: sched,
		stopCh:    make(chan struct{}),
	}, nil
}

// Start starts the proxy server
func (p *ProxyServer) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("proxy server is already running")
	}

	// Create HTTP handler
	mux := http.NewServeMux()
	p.registerHandlers(mux)

	// Create HTTP server
	addr := fmt.Sprintf(":%d", p.config.Port)
	p.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second, // Long timeout for streaming
		IdleTimeout:  120 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	// Start server in goroutine after the listener is bound, so Start can surface port conflicts.
	go func() {
		log.Printf("[WaveProxy] Starting server on port %d", p.config.Port)
		if err := p.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("[WaveProxy] Server error: %v", err)
		}
	}()

	p.running = true
	p.startedAt = time.Now()
	p.stopCh = make(chan struct{})

	log.Printf("[WaveProxy] Server started successfully")
	return nil
}

// Stop stops the proxy server
func (p *ProxyServer) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return fmt.Errorf("proxy server is not running")
	}

	log.Printf("[WaveProxy] Stopping server...")

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := p.server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[WaveProxy] Error during shutdown: %v", err)
	}

	// Stop metrics manager
	p.metrics.Stop()

	// Stop session manager
	p.sessions.Stop()

	// Stop history manager
	p.history.Stop()

	close(p.stopCh)
	p.running = false

	log.Printf("[WaveProxy] Server stopped")
	return nil
}

// Status returns the current server status
func (p *ProxyServer) Status() ProxyStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	status := ProxyStatus{
		Running:      p.running,
		Port:         p.config.Port,
		Version:      Version,
		ChannelCount: p.channels.Count(),
	}

	if p.running {
		status.StartedAt = p.startedAt
		status.Uptime = time.Since(p.startedAt).Round(time.Second).String()
	}

	return status
}

// IsRunning returns whether the server is running
func (p *ProxyServer) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// GetConfig returns the current configuration
func (p *ProxyServer) GetConfig() *config.Config {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// GetChannelManager returns the channel manager
func (p *ProxyServer) GetChannelManager() *channel.Manager {
	return p.channels
}

// GetMetricsManager returns the metrics manager
func (p *ProxyServer) GetMetricsManager() *metrics.Manager {
	return p.metrics
}

// GetScheduler returns the scheduler
func (p *ProxyServer) GetScheduler() *scheduler.Scheduler {
	return p.scheduler
}

// GetHistoryManager returns the history manager
func (p *ProxyServer) GetHistoryManager() *history.Manager {
	return p.history
}

// registerHandlers registers all HTTP handlers
func (p *ProxyServer) registerHandlers(mux *http.ServeMux) {
	// Health check endpoint
	mux.HandleFunc("/health", handler.HealthHandler(p.config))

	// Claude Messages API
	mux.HandleFunc("/v1/messages", handler.MessagesHandler(p.config, p.scheduler, p.metrics, p.history))
	mux.HandleFunc("/v1/messages/count_tokens", handler.CountTokensHandler(p.config))

	// Responses API
	mux.HandleFunc("/v1/responses", handler.ResponsesHandler(p.config, p.scheduler, p.metrics, p.sessions, p.history))
	mux.HandleFunc("/v1/responses/compact", handler.ResponsesCompactHandler(p.config, p.scheduler, p.metrics, p.sessions, p.history))

	// OpenAI compatible aliases (some clients treat base_url as .../v1 and call /responses, /models, etc).
	mux.HandleFunc("/messages", handler.MessagesHandler(p.config, p.scheduler, p.metrics, p.history))
	mux.HandleFunc("/messages/count_tokens", handler.CountTokensHandler(p.config))
	mux.HandleFunc("/responses", handler.ResponsesHandler(p.config, p.scheduler, p.metrics, p.sessions, p.history))
	mux.HandleFunc("/responses/compact", handler.ResponsesCompactHandler(p.config, p.scheduler, p.metrics, p.sessions, p.history))

	// Models API (OpenAI compatible)
	mux.HandleFunc("/v1/models", handler.ModelsHandler(p.config, p.scheduler))
	mux.HandleFunc("/v1/models/", handler.ModelsDetailHandler(p.config, p.scheduler))
	mux.HandleFunc("/models", handler.ModelsHandler(p.config, p.scheduler))
	mux.HandleFunc("/models/", handler.ModelsDetailHandler(p.config, p.scheduler))

	// Gemini API (wildcard pattern)
	mux.HandleFunc("/v1beta/models/", handler.GeminiHandler(p.config, p.scheduler, p.metrics, p.history))

	// Fallback: log unknown routes
	mux.HandleFunc("/", handler.NotFoundHandler())
}

// ReloadConfig reloads the configuration
func (p *ProxyServer) ReloadConfig(cfg *config.Config) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	p.config = cfg
	log.Printf("[WaveProxy] Configuration reloaded")
	return nil
}
