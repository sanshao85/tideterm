// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package waveproxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sanshao85/tideterm/pkg/wavebase"
	"github.com/sanshao85/tideterm/pkg/waveproxy/config"
)

const proxyConfigFilename = "waveproxy.json"

func getProxyConfigPath() (string, error) {
	if err := wavebase.EnsureWaveConfigDir(); err != nil {
		return "", err
	}
	return filepath.Join(wavebase.GetWaveConfigDir(), proxyConfigFilename), nil
}

func loadProxyConfigFromDisk() (*config.Config, error) {
	path, err := getProxyConfigPath()
	if err != nil {
		return nil, err
	}
	log.Printf("[WaveProxy-Persist] Loading config from: %s", path)
	cfg, err := config.LoadConfig(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("[WaveProxy-Persist] Config file not found, using defaults")
			return config.DefaultConfig(), nil
		}
		log.Printf("[WaveProxy-Persist] Error loading config: %v", err)
		return nil, err
	}
	log.Printf("[WaveProxy-Persist] Config loaded: %d channels, %d response channels, %d gemini channels",
		len(cfg.Channels), len(cfg.ResponseChannels), len(cfg.GeminiChannels))
	return cfg, nil
}

func saveProxyConfigToDisk(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}
	path, err := getProxyConfigPath()
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, proxyConfigFilename+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Chmod(0600); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}

