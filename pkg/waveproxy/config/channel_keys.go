// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package config

import "strings"

// EnabledAPIKeys returns the list of enabled, non-empty API key strings for this channel.
func (ch *Channel) EnabledAPIKeys() []string {
	if ch == nil || len(ch.APIKeys) == 0 {
		return nil
	}
	out := make([]string, 0, len(ch.APIKeys))
	for _, k := range ch.APIKeys {
		if !k.Enabled {
			continue
		}
		key := strings.TrimSpace(k.Key)
		if key == "" {
			continue
		}
		out = append(out, key)
	}
	return out
}

// EnabledAPIKeyCount returns the number of enabled, non-empty API keys.
func (ch *Channel) EnabledAPIKeyCount() int {
	if ch == nil || len(ch.APIKeys) == 0 {
		return 0
	}
	n := 0
	for _, k := range ch.APIKeys {
		if !k.Enabled {
			continue
		}
		if strings.TrimSpace(k.Key) == "" {
			continue
		}
		n++
	}
	return n
}

