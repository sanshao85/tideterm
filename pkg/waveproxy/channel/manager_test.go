package channel

import (
	"testing"

	"github.com/sanshao85/tideterm/pkg/waveproxy/config"
)

func TestManagerResponsesPrefersOpenAIChannels(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Channels = []config.Channel{
		{ID: "claude1", Name: "Claude", ServiceType: "claude", BaseURL: "https://example.com/api"},
		{ID: "openai1", Name: "OpenAI", ServiceType: "openai", BaseURL: "https://example.com/openai/v1"},
	}
	cfg.ResponseChannels = nil

	mgr := NewManager()
	mgr.LoadChannels(cfg)

	active := mgr.GetActiveChannels(ChannelTypeResponses)
	if len(active) != 1 {
		t.Fatalf("expected 1 active responses channel, got %d", len(active))
	}
	if active[0].ID != "openai1" {
		t.Fatalf("expected openai1 to be selected for responses, got %q", active[0].ID)
	}
}

func TestManagerMessagesFiltersOutOpenAIChannels(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Channels = []config.Channel{
		{ID: "openai1", Name: "OpenAI", ServiceType: "openai", BaseURL: "https://example.com/openai/v1"},
		{ID: "claude1", Name: "Claude", ServiceType: "claude", BaseURL: "https://example.com/api"},
	}

	mgr := NewManager()
	mgr.LoadChannels(cfg)

	active := mgr.GetActiveChannels(ChannelTypeMessages)
	if len(active) != 1 {
		t.Fatalf("expected 1 active messages channel, got %d", len(active))
	}
	if active[0].ID != "claude1" {
		t.Fatalf("expected claude1 to be selected for messages, got %q", active[0].ID)
	}
}

func TestManagerResponsesDefaultsServiceTypeForResponseChannels(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ResponseChannels = []config.Channel{
		{ID: "r1", Name: "RespChannel", ServiceType: "", BaseURL: "https://example.com/openai"},
	}

	mgr := NewManager()
	mgr.LoadChannels(cfg)

	active := mgr.GetActiveChannels(ChannelTypeResponses)
	if len(active) != 1 {
		t.Fatalf("expected 1 active responses channel, got %d", len(active))
	}
	if active[0].ID != "r1" {
		t.Fatalf("expected r1 to be selected for responses, got %q", active[0].ID)
	}
}

