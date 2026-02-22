// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"strings"
)

// APIKey represents a single API key entry with an enabled/disabled state.
//
// Backward compatibility:
// - Accepts either a JSON string ("sk-...") or an object {"key":"sk-...","enabled":true}.
// - When unmarshalling from a string, Enabled defaults to true.
// - When unmarshalling from an object without "enabled", Enabled defaults to true.
type APIKey struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

func (k *APIKey) UnmarshalJSON(data []byte) error {
	// Accept legacy string element: "sk-..."
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		k.Key = strings.TrimSpace(s)
		k.Enabled = k.Key != ""
		return nil
	}

	// Accept object element.
	var obj struct {
		Key     string `json:"key"`
		Enabled *bool  `json:"enabled,omitempty"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	k.Key = strings.TrimSpace(obj.Key)
	if obj.Enabled != nil {
		k.Enabled = *obj.Enabled
	} else {
		k.Enabled = k.Key != ""
	}
	return nil
}

func (k APIKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Key     string `json:"key"`
		Enabled bool   `json:"enabled"`
	}{
		Key:     strings.TrimSpace(k.Key),
		Enabled: k.Enabled,
	})
}

