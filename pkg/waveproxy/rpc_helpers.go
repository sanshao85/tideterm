// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package waveproxy

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sanshao85/tideterm/pkg/waveproxy/config"
)

type rpcProxyChannel struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	ServiceType        string            `json:"serviceType"`
	BaseUrl            string            `json:"baseUrl"`
	BaseUrls           []string          `json:"baseUrls,omitempty"`
	ApiKeys            []config.APIKey   `json:"apiKeys"`
	AuthType           string            `json:"authType,omitempty"`
	Priority           int               `json:"priority"`
	Status             string            `json:"status"`
	PromotionUntil     string            `json:"promotionUntil,omitempty"`
	ModelMapping       map[string]string `json:"modelMapping,omitempty"`
	LowQuality         bool              `json:"lowQuality,omitempty"`
	InsecureSkipVerify bool              `json:"insecureSkipVerify,omitempty"`
	Description        string            `json:"description,omitempty"`
}

func decodeProxyChannel(input interface{}) (*config.Channel, error) {
	if input == nil {
		return nil, fmt.Errorf("channel cannot be nil")
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var rpcCh rpcProxyChannel
	if err := json.Unmarshal(data, &rpcCh); err != nil {
		return nil, err
	}

	var promotionUntil *time.Time
	if rpcCh.PromotionUntil != "" {
		parsed, err := time.Parse(time.RFC3339, rpcCh.PromotionUntil)
		if err != nil {
			return nil, fmt.Errorf("invalid promotionUntil: %w", err)
		}
		promotionUntil = &parsed
	}

	channelId := strings.TrimSpace(rpcCh.ID)
	if channelId == "" {
		channelId = uuid.NewString()
	}

	ch := &config.Channel{
		ID:                 channelId,
		Name:               strings.TrimSpace(rpcCh.Name),
		ServiceType:        strings.TrimSpace(rpcCh.ServiceType),
		BaseURL:            strings.TrimSpace(rpcCh.BaseUrl),
		BaseURLs:           rpcCh.BaseUrls,
		APIKeys:            rpcCh.ApiKeys,
		AuthType:           strings.TrimSpace(rpcCh.AuthType),
		Priority:           rpcCh.Priority,
		Status:             strings.TrimSpace(rpcCh.Status),
		PromotionUntil:     promotionUntil,
		ModelMapping:       rpcCh.ModelMapping,
		LowQuality:         rpcCh.LowQuality,
		InsecureSkipVerify: rpcCh.InsecureSkipVerify,
		Description:        strings.TrimSpace(rpcCh.Description),
	}

	if ch.Status == "" {
		ch.Status = "active"
	}

	return ch, nil
}

func configSliceForChannelType(cfg *config.Config, channelType string) (*[]config.Channel, error) {
	switch channelType {
	case "messages", "":
		return &cfg.Channels, nil
	case "responses":
		return &cfg.ResponseChannels, nil
	case "gemini":
		return &cfg.GeminiChannels, nil
	default:
		return nil, fmt.Errorf("unknown channel type: %s", channelType)
	}
}

func countConfigChannels(cfg *config.Config) int {
	if cfg == nil {
		return 0
	}
	return len(cfg.Channels) + len(cfg.ResponseChannels) + len(cfg.GeminiChannels)
}

func pingBaseURL(baseURL string, insecureSkipVerify bool) (int64, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return 0, fmt.Errorf("empty base URL")
	}
	if !strings.Contains(baseURL, "://") {
		baseURL = "https://" + baseURL
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return 0, err
	}

	host := parsed.Host
	if host == "" {
		return 0, fmt.Errorf("invalid base URL: missing host")
	}

	scheme := parsed.Scheme
	if scheme == "" {
		scheme = "https"
	}

	port := parsed.Port()
	hostNoPort := strings.TrimSuffix(host, ":"+port)
	if port == "" {
		if scheme == "http" {
			port = "80"
		} else {
			port = "443"
		}
	}
	addr := net.JoinHostPort(hostNoPort, port)

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	start := time.Now()

	if scheme == "http" {
		conn, err := dialer.Dial("tcp", addr)
		if err != nil {
			return 0, err
		}
		_ = conn.Close()
		return time.Since(start).Milliseconds(), nil
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		InsecureSkipVerify: insecureSkipVerify,
		ServerName:         hostNoPort,
		MinVersion:         tls.VersionTLS12,
	})
	if err != nil {
		return 0, err
	}
	_ = conn.Close()
	return time.Since(start).Milliseconds(), nil
}
