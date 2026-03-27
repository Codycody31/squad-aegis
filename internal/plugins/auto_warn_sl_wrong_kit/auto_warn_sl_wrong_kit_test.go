package auto_warn_sl_wrong_kit

import (
	"testing"

	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
)

func TestFindTrackedPlayerKeyLockedSupportsSteamOnlyEOSOnlyAndDualIDs(t *testing.T) {
	const steamID = "76561198000000042"
	const eosID = "abcdef0123456789abcdef0123456789"

	t.Run("steam only", func(t *testing.T) {
		tracker := &PlayerTracker{Player: &plugin_manager.PlayerInfo{SteamID: steamID}}
		plugin := &AutoWarnSLWrongKitPlugin{
			trackedPlayers: map[string]*PlayerTracker{
				steamID: tracker,
			},
		}

		key, gotTracker, ok := plugin.findTrackedPlayerKeyLocked(steamID, steamID, "")
		if !ok {
			t.Fatalf("expected tracker to be found")
		}
		if key != steamID {
			t.Fatalf("key = %q, want %q", key, steamID)
		}
		if gotTracker != tracker {
			t.Fatalf("unexpected tracker returned")
		}
	})

	t.Run("eos only", func(t *testing.T) {
		tracker := &PlayerTracker{Player: &plugin_manager.PlayerInfo{EOSID: eosID}}
		plugin := &AutoWarnSLWrongKitPlugin{
			trackedPlayers: map[string]*PlayerTracker{
				eosID: tracker,
			},
		}

		key, gotTracker, ok := plugin.findTrackedPlayerKeyLocked(eosID, "", eosID)
		if !ok {
			t.Fatalf("expected tracker to be found")
		}
		if key != eosID {
			t.Fatalf("key = %q, want %q", key, eosID)
		}
		if gotTracker != tracker {
			t.Fatalf("unexpected tracker returned")
		}
	})

	t.Run("dual ids find tracker stored under alternate key", func(t *testing.T) {
		tracker := &PlayerTracker{Player: &plugin_manager.PlayerInfo{SteamID: steamID, EOSID: eosID}}
		plugin := &AutoWarnSLWrongKitPlugin{
			trackedPlayers: map[string]*PlayerTracker{
				eosID: tracker,
			},
		}

		key, gotTracker, ok := plugin.findTrackedPlayerKeyLocked(steamID, steamID, eosID)
		if !ok {
			t.Fatalf("expected tracker to be found")
		}
		if key != eosID {
			t.Fatalf("key = %q, want %q", key, eosID)
		}
		if gotTracker != tracker {
			t.Fatalf("unexpected tracker returned")
		}
	})
}
