package discord_admin_cam_logs

import "testing"

func TestFindAdminCamSessionKeyLockedSupportsSteamOnlyEOSOnlyAndDualIDs(t *testing.T) {
	const steamID = "76561198000000042"
	const eosID = "abcdef0123456789abcdef0123456789"

	t.Run("steam only", func(t *testing.T) {
		session := &AdminCamSession{SteamID: steamID}
		plugin := &DiscordAdminCamLogsPlugin{
			adminsInCam: map[string]*AdminCamSession{
				steamID: session,
			},
		}

		key, gotSession, ok := plugin.findAdminCamSessionKeyLocked(steamID, "")
		if !ok {
			t.Fatalf("expected session to be found")
		}
		if key != steamID {
			t.Fatalf("key = %q, want %q", key, steamID)
		}
		if gotSession != session {
			t.Fatalf("unexpected session returned")
		}
	})

	t.Run("eos only", func(t *testing.T) {
		session := &AdminCamSession{EosID: eosID}
		plugin := &DiscordAdminCamLogsPlugin{
			adminsInCam: map[string]*AdminCamSession{
				eosID: session,
			},
		}

		key, gotSession, ok := plugin.findAdminCamSessionKeyLocked("", eosID)
		if !ok {
			t.Fatalf("expected session to be found")
		}
		if key != eosID {
			t.Fatalf("key = %q, want %q", key, eosID)
		}
		if gotSession != session {
			t.Fatalf("unexpected session returned")
		}
	})

	t.Run("dual ids find session stored under alternate key", func(t *testing.T) {
		session := &AdminCamSession{SteamID: steamID, EosID: eosID}
		plugin := &DiscordAdminCamLogsPlugin{
			adminsInCam: map[string]*AdminCamSession{
				eosID: session,
			},
		}

		key, gotSession, ok := plugin.findAdminCamSessionKeyLocked(steamID, "")
		if !ok {
			t.Fatalf("expected session to be found")
		}
		if key != eosID {
			t.Fatalf("key = %q, want %q", key, eosID)
		}
		if gotSession != session {
			t.Fatalf("unexpected session returned")
		}
	})
}
