package whitelistprogress

import (
	"errors"
	"testing"
	"time"
)

func TestParseStateRoundTrip(t *testing.T) {
	raw, err := MarshalPlayers(map[string]*PlayerRecord{
		"76561198000000021": {
			PlayerID:         "76561198000000021",
			QualifiedSeconds: 7200,
			LifetimeSeconds:  14400,
			LastEarnedAt:     time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			LastSeenAt:       time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}

	state, err := ParseState(string(raw))
	if err != nil {
		t.Fatalf("parse state: %v", err)
	}

	if state.Version != CurrentVersion {
		t.Fatalf("version = %d, want %d", state.Version, CurrentVersion)
	}

	record := state.Players["76561198000000021"]
	if record == nil {
		t.Fatalf("expected parsed record")
	}

	if got, want := record.QualifiedSeconds, int64(7200); got != want {
		t.Fatalf("qualified seconds = %d, want %d", got, want)
	}
}

func TestParseStateRejectsLegacyPayload(t *testing.T) {
	legacy := `{"76561198000000022":{"steam_id":"76561198000000022","progress":50}}`

	_, err := ParseState(legacy)
	if !errors.Is(err, ErrUnknownFormat) {
		t.Fatalf("expected ErrUnknownFormat, got %v", err)
	}
}

func TestParseStateSupportsLegacyPlayerRecordIdentifier(t *testing.T) {
	state, err := ParseState(`{"version":1,"players":{"ABCDEF0123456789ABCDEF0123456789":{"steam_id":"ABCDEF0123456789ABCDEF0123456789","qualified_seconds":60,"lifetime_seconds":120}}}`)
	if err != nil {
		t.Fatalf("parse state: %v", err)
	}

	record := state.Players["abcdef0123456789abcdef0123456789"]
	if record == nil {
		t.Fatalf("expected normalized EOS record")
	}

	if got, want := record.PlayerID, "abcdef0123456789abcdef0123456789"; got != want {
		t.Fatalf("player ID = %q, want %q", got, want)
	}
}

func TestEnsureRecordMergesEOSAndSteamObservations(t *testing.T) {
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	players := map[string]*PlayerRecord{
		"abcdef0123456789abcdef0123456789": {
			PlayerID:         "abcdef0123456789abcdef0123456789",
			EOSID:            "abcdef0123456789abcdef0123456789",
			QualifiedSeconds: 3600,
			LifetimeSeconds:  7200,
			LastEarnedAt:     now.Add(-time.Hour),
			LastSeenAt:       now.Add(-time.Hour),
		},
	}

	record := EnsureRecord(players, "76561198000000021", "abcdef0123456789ABCDEF0123456789", now)
	if record == nil {
		t.Fatalf("expected merged record")
	}

	if got, want := record.PlayerID, "76561198000000021"; got != want {
		t.Fatalf("canonical player ID = %q, want %q", got, want)
	}
	if got, want := record.SteamID, "76561198000000021"; got != want {
		t.Fatalf("steam ID = %q, want %q", got, want)
	}
	if got, want := record.EOSID, "abcdef0123456789abcdef0123456789"; got != want {
		t.Fatalf("eos ID = %q, want %q", got, want)
	}

	lookup, ok := FindRecord(players, "76561198000000021")
	if !ok || lookup != record {
		t.Fatalf("expected Steam lookup to resolve merged record")
	}

	if len(players) != 1 {
		t.Fatalf("expected one canonical record after merge, got %d", len(players))
	}

	if _, exists := players["abcdef0123456789abcdef0123456789"]; exists {
		t.Fatalf("expected EOS key to be replaced by Steam canonical key")
	}
}

func TestEnsureRecordSupportsSteamOnlyEOSOnlyAndBoth(t *testing.T) {
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		steamID      string
		eosID        string
		wantPlayerID string
		wantSteamID  string
		wantEOSID    string
	}{
		{
			name:         "steam only",
			steamID:      "76561198000000031",
			wantPlayerID: "76561198000000031",
			wantSteamID:  "76561198000000031",
		},
		{
			name:         "eos only",
			eosID:        "abcdef0123456789abcdef0123456799",
			wantPlayerID: "abcdef0123456789abcdef0123456799",
			wantEOSID:    "abcdef0123456789abcdef0123456799",
		},
		{
			name:         "steam and eos",
			steamID:      "76561198000000032",
			eosID:        "abcdef0123456789abcdef012345679a",
			wantPlayerID: "76561198000000032",
			wantSteamID:  "76561198000000032",
			wantEOSID:    "abcdef0123456789abcdef012345679a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			players := map[string]*PlayerRecord{}

			record := EnsureRecord(players, tt.steamID, tt.eosID, now)
			if record == nil {
				t.Fatalf("expected record")
			}

			if got := record.PlayerID; got != tt.wantPlayerID {
				t.Fatalf("player ID = %q, want %q", got, tt.wantPlayerID)
			}
			if got := record.SteamID; got != tt.wantSteamID {
				t.Fatalf("steam ID = %q, want %q", got, tt.wantSteamID)
			}
			if got := record.EOSID; got != tt.wantEOSID {
				t.Fatalf("eos ID = %q, want %q", got, tt.wantEOSID)
			}

			if tt.steamID != "" {
				lookup, ok := FindRecord(players, tt.steamID)
				if !ok || lookup != record {
					t.Fatalf("expected steam lookup to resolve record")
				}
			}
			if tt.eosID != "" {
				lookup, ok := FindRecord(players, tt.eosID)
				if !ok || lookup != record {
					t.Fatalf("expected eos lookup to resolve record")
				}
			}
		})
	}
}

func TestFindRecordByIdentifiersResolvesLegacyEOSRecordFromSteamLookup(t *testing.T) {
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	players := map[string]*PlayerRecord{
		"abcdef0123456789abcdef0123456789": {
			PlayerID:         "abcdef0123456789abcdef0123456789",
			EOSID:            "abcdef0123456789abcdef0123456789",
			QualifiedSeconds: 3600,
			LifetimeSeconds:  7200,
			LastEarnedAt:     now.Add(-time.Hour),
			LastSeenAt:       now.Add(-time.Hour),
		},
	}

	record, ok := FindRecordByIdentifiers(players, "76561198000000021", "ABCDEF0123456789ABCDEF0123456789")
	if !ok || record == nil {
		t.Fatalf("expected identifier pair lookup to resolve legacy EOS record")
	}

	if got, want := record.EOSID, "abcdef0123456789abcdef0123456789"; got != want {
		t.Fatalf("eos ID = %q, want %q", got, want)
	}
}

func TestParseStateCanonicalizesRecordKeyToSteamWhenAvailable(t *testing.T) {
	state, err := ParseState(`{"version":1,"players":{"ABCDEF0123456789ABCDEF0123456789":{"player_id":"ABCDEF0123456789ABCDEF0123456789","steam_id":"76561198000000021","eos_id":"ABCDEF0123456789ABCDEF0123456789","qualified_seconds":60,"lifetime_seconds":120}}}`)
	if err != nil {
		t.Fatalf("parse state: %v", err)
	}

	record := state.Players["76561198000000021"]
	if record == nil {
		t.Fatalf("expected Steam-canonical record")
	}

	if got, want := record.PlayerID, "76561198000000021"; got != want {
		t.Fatalf("player ID = %q, want %q", got, want)
	}

	if _, exists := state.Players["abcdef0123456789abcdef0123456789"]; exists {
		t.Fatalf("expected EOS map key to be canonicalized to Steam")
	}
}

func TestLegacyPercentToSeconds(t *testing.T) {
	if got, want := LegacyPercentToSeconds(50, 6), int64(3*time.Hour/time.Second); got != want {
		t.Fatalf("legacy conversion = %d, want %d", got, want)
	}
}
