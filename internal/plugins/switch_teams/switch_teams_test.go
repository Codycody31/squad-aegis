package switch_teams

import (
	"testing"
	"time"
)

func TestResolveLastSwitchKeyLockedSupportsSteamOnlyEOSOnlyAndDualIDs(t *testing.T) {
	const steamID = "76561198000000042"
	const eosID = "abcdef0123456789abcdef0123456789"

	now := time.Now()

	tests := []struct {
		name        string
		playerID    string
		steamID     string
		eosID       string
		existingKey string
		expectedKey string
	}{
		{
			name:        "steam only",
			playerID:    steamID,
			steamID:     steamID,
			existingKey: steamID,
			expectedKey: steamID,
		},
		{
			name:        "eos only",
			playerID:    eosID,
			eosID:       eosID,
			existingKey: eosID,
			expectedKey: eosID,
		},
		{
			name:        "dual ids migrate eos key to steam key",
			playerID:    steamID,
			steamID:     steamID,
			eosID:       eosID,
			existingKey: eosID,
			expectedKey: steamID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &SwitchTeamsPlugin{
				lastSwitches: map[string]time.Time{
					tt.existingKey: now,
				},
			}

			key := plugin.resolveLastSwitchKeyLocked(tt.playerID, tt.steamID, tt.eosID)
			if key != tt.expectedKey {
				t.Fatalf("resolveLastSwitchKeyLocked() = %q, want %q", key, tt.expectedKey)
			}

			gotTime, ok := plugin.lastSwitches[tt.expectedKey]
			if !ok {
				t.Fatalf("expected cooldown entry for key %q", tt.expectedKey)
			}
			if !gotTime.Equal(now) {
				t.Fatalf("cooldown time mismatch")
			}

			if tt.existingKey != tt.expectedKey {
				if _, ok := plugin.lastSwitches[tt.existingKey]; ok {
					t.Fatalf("expected old key %q to be removed", tt.existingKey)
				}
			}
		})
	}
}
