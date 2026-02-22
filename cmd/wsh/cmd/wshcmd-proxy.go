// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/sanshao85/tideterm/pkg/waveproxy"
	"github.com/sanshao85/tideterm/pkg/waveproxy/config"
)

var proxyPort int
var proxyConfigFile string

func init() {
	rootCmd.AddCommand(proxyCmd)
	proxyCmd.Flags().IntVarP(&proxyPort, "port", "p", 3000, "port to listen on")
	proxyCmd.Flags().StringVarP(&proxyConfigFile, "config", "c", "", "config file path")
}

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "start the AI API proxy server",
	Long: `Start the WaveProxy AI API proxy server.

This command starts a local HTTP proxy server that can forward requests
to multiple AI API providers (Claude, OpenAI, Gemini) with load balancing,
failover, and metrics collection.

Examples:
  wsh proxy                    # Start proxy on default port 3000
  wsh proxy --port 8080        # Start proxy on port 8080
  wsh proxy -p 8080 -c config.json  # Start with custom config file
`,
	RunE: proxyRun,
}

func proxyRun(cmd *cobra.Command, args []string) error {
	// Create config
	cfg := config.DefaultConfig()
	cfg.Port = proxyPort

	// Load config file if provided
	if proxyConfigFile != "" {
		loadedCfg, err := config.LoadConfig(proxyConfigFile)
		if err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
		cfg = loadedCfg
		// Override port if specified via flag
		if cmd.Flags().Changed("port") {
			cfg.Port = proxyPort
		}
	}

	// Create proxy server
	server, err := waveproxy.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create proxy server: %w", err)
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the server
	if err := server.Start(ctx); err != nil {
		return fmt.Errorf("failed to start proxy server: %w", err)
	}

	log.Printf("[WaveProxy] Server running on port %d", cfg.Port)
	log.Printf("[WaveProxy] Press Ctrl+C to stop")

	// Wait for shutdown signal
	<-sigChan
	log.Printf("[WaveProxy] Received shutdown signal")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Stop(shutdownCtx); err != nil {
		log.Printf("[WaveProxy] Error during shutdown: %v", err)
	}

	log.Printf("[WaveProxy] Server stopped")
	return nil
}

// proxyHealthCheck performs a health check on the running proxy
func proxyHealthCheck(port int) error {
	url := fmt.Sprintf("http://localhost:%d/health", port)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}
