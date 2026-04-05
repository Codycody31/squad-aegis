package plugin_manager

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestMissingRequiredCapabilitiesSupportsScopedPluginAPIs(t *testing.T) {
	t.Parallel()

	if missing := missingRequiredCapabilities([]string{
		NativePluginCapabilityAPIRule,
		NativePluginCapabilityAPIDiscord,
		NativePluginCapabilityAPIConnector,
		NativePluginCapabilityAPIRCON,
	}); len(missing) != 0 {
		t.Fatalf("missingRequiredCapabilities() = %v, want no missing capabilities", missing)
	}

	if missing := missingRequiredCapabilities([]string{"api.fake"}); len(missing) != 1 || missing[0] != "api.fake" {
		t.Fatalf("missingRequiredCapabilities() = %v, want [api.fake]", missing)
	}
}

func TestNativePluginHostCapabilitiesIncludesScopedPluginAPIs(t *testing.T) {
	t.Parallel()

	capabilities := NativePluginHostCapabilities()
	want := map[string]bool{
		NativePluginCapabilityAPIDatabase:  true,
		NativePluginCapabilityAPIRule:      true,
		NativePluginCapabilityAPIDiscord:   true,
		NativePluginCapabilityAPIConnector: true,
	}

	for _, capability := range capabilities {
		delete(want, capability)
	}

	if len(want) != 0 {
		t.Fatalf("NativePluginHostCapabilities() missing expected capabilities: %v", want)
	}
}

func TestValidateManagedAdminRecordOwnership(t *testing.T) {
	t.Parallel()

	pluginInstanceID := uuid.New()
	otherPluginInstanceID := uuid.New()

	tests := []struct {
		name    string
		records []managedAdminRecord
		wantIDs int
		wantErr string
	}{
		{
			name: "owned records are accepted",
			records: []managedAdminRecord{
				{
					ID: uuid.New(),
					ManagedByPluginInstance: sql.NullString{
						String: pluginInstanceID.String(),
						Valid:  true,
					},
				},
				{
					ID: uuid.New(),
					ManagedByPluginInstance: sql.NullString{
						String: pluginInstanceID.String(),
						Valid:  true,
					},
				},
			},
			wantIDs: 2,
		},
		{
			name: "unmanaged records are rejected",
			records: []managedAdminRecord{
				{
					ID:                      uuid.New(),
					ManagedByPluginInstance: sql.NullString{},
				},
			},
			wantErr: "not managed by this plugin",
		},
		{
			name: "records owned by another plugin are rejected",
			records: []managedAdminRecord{
				{
					ID: uuid.New(),
					ManagedByPluginInstance: sql.NullString{
						String: otherPluginInstanceID.String(),
						Valid:  true,
					},
				},
			},
			wantErr: "managed by another plugin",
		},
		{
			name: "invalid owner IDs are rejected",
			records: []managedAdminRecord{
				{
					ID: uuid.New(),
					ManagedByPluginInstance: sql.NullString{
						String: "not-a-uuid",
						Valid:  true,
					},
				},
			},
			wantErr: "invalid plugin ownership",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ids, err := validateManagedAdminRecordOwnership(tt.records, pluginInstanceID)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("validateManagedAdminRecordOwnership() error = nil, want %q", tt.wantErr)
				}
				if got := err.Error(); !strings.Contains(got, tt.wantErr) {
					t.Fatalf("validateManagedAdminRecordOwnership() error = %q, want substring %q", got, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("validateManagedAdminRecordOwnership() error = %v", err)
			}
			if len(ids) != tt.wantIDs {
				t.Fatalf("validateManagedAdminRecordOwnership() len = %d, want %d", len(ids), tt.wantIDs)
			}
		})
	}
}

func TestResolvePlayerIdentifiersMergesOnlinePlayerIDs(t *testing.T) {
	t.Parallel()

	steamID, eosID, normalizedPlayerID, err := resolvePlayerIdentifiers(
		"ABCDEF0123456789ABCDEF0123456789",
		[]*PlayerInfo{
			{
				SteamID: "76561198000000021",
				EOSID:   "abcdef0123456789abcdef0123456789",
			},
		},
	)
	if err != nil {
		t.Fatalf("resolvePlayerIdentifiers returned error: %v", err)
	}

	if got, want := steamID, "76561198000000021"; got != want {
		t.Fatalf("steam ID = %q, want %q", got, want)
	}
	if got, want := eosID, "abcdef0123456789abcdef0123456789"; got != want {
		t.Fatalf("eos ID = %q, want %q", got, want)
	}
	if got, want := normalizedPlayerID, "76561198000000021"; got != want {
		t.Fatalf("normalized player ID = %q, want %q", got, want)
	}
}

func TestResolvePlayerIdentifiersSupportsSteamOnlyEOSOnlyAndBoth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		playerID     string
		players      []*PlayerInfo
		wantSteamID  string
		wantEOSID    string
		wantResolved string
	}{
		{
			name:         "steam only",
			playerID:     "76561198000000041",
			wantSteamID:  "76561198000000041",
			wantResolved: "76561198000000041",
		},
		{
			name:         "eos only",
			playerID:     "abcdef0123456789abcdef01234567aa",
			wantEOSID:    "abcdef0123456789abcdef01234567aa",
			wantResolved: "abcdef0123456789abcdef01234567aa",
		},
		{
			name:     "steam and eos",
			playerID: "abcdef0123456789abcdef01234567ab",
			players: []*PlayerInfo{
				{
					SteamID: "76561198000000042",
					EOSID:   "abcdef0123456789abcdef01234567ab",
				},
			},
			wantSteamID:  "76561198000000042",
			wantEOSID:    "abcdef0123456789abcdef01234567ab",
			wantResolved: "76561198000000042",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			steamID, eosID, normalizedPlayerID, err := resolvePlayerIdentifiers(tt.playerID, tt.players)
			if err != nil {
				t.Fatalf("resolvePlayerIdentifiers returned error: %v", err)
			}

			if got := steamID; got != tt.wantSteamID {
				t.Fatalf("steam ID = %q, want %q", got, tt.wantSteamID)
			}
			if got := eosID; got != tt.wantEOSID {
				t.Fatalf("eos ID = %q, want %q", got, tt.wantEOSID)
			}
			if got := normalizedPlayerID; got != tt.wantResolved {
				t.Fatalf("normalized player ID = %q, want %q", got, tt.wantResolved)
			}
		})
	}
}
