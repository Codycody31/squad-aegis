package squad_creation_blocker

import "testing"

func TestResolvePlayerAttemptKeyLockedSupportsSteamOnlyEOSOnlyAndDualIDs(t *testing.T) {
	const steamID = "76561198000000042"
	const eosID = "abcdef0123456789abcdef0123456789"

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
			attemptData := &PlayerAttemptData{AttemptCount: 1}
			plugin := &SquadCreationBlockerPlugin{
				playerAttempts: map[string]*PlayerAttemptData{
					tt.existingKey: attemptData,
				},
			}

			key := plugin.resolvePlayerAttemptKeyLocked(tt.playerID, tt.steamID, tt.eosID)
			if key != tt.expectedKey {
				t.Fatalf("resolvePlayerAttemptKeyLocked() = %q, want %q", key, tt.expectedKey)
			}

			gotData, ok := plugin.playerAttempts[tt.expectedKey]
			if !ok {
				t.Fatalf("expected attempt data for key %q", tt.expectedKey)
			}
			if gotData != attemptData {
				t.Fatalf("unexpected attempt data returned")
			}

			if tt.existingKey != tt.expectedKey {
				if _, ok := plugin.playerAttempts[tt.existingKey]; ok {
					t.Fatalf("expected old key %q to be removed", tt.existingKey)
				}
			}
		})
	}
}
