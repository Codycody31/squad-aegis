package server

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"go.codycody31.dev/squad-aegis/internal/models"
)

func TestShouldPreserveExistingBansCfgEntry(t *testing.T) {
	t.Parallel()

	eosID := "abcdef0123456789abcdef0123456789"

	tests := []struct {
		name             string
		entry            models.CfgBanEntry
		managedSteamIDs  map[string]bool
		managedEOSIDs    map[string]bool
		excludedSteamIDs map[string]bool
		excludedEOSIDs   map[string]bool
		want             bool
	}{
		{
			name:  "preserves unmanaged active steam ban",
			entry: models.CfgBanEntry{SteamID: "76561198000000001"},
			want:  true,
		},
		{
			name:            "drops managed subscribed steam ban",
			entry:           models.CfgBanEntry{SteamID: "76561198000000001"},
			managedSteamIDs: map[string]bool{"76561198000000001": true},
			want:            false,
		},
		{
			name:             "drops explicitly removed steam ban",
			entry:            models.CfgBanEntry{SteamID: "76561198000000001"},
			excludedSteamIDs: map[string]bool{"76561198000000001": true},
			want:             false,
		},
		{
			name:          "drops managed eos ban",
			entry:         models.CfgBanEntry{EOSID: strings.ToUpper(eosID)},
			managedEOSIDs: map[string]bool{eosID: true},
			want:          false,
		},
		{
			name:           "drops explicitly removed eos ban",
			entry:          models.CfgBanEntry{EOSID: strings.ToUpper(eosID)},
			excludedEOSIDs: map[string]bool{eosID: true},
			want:           false,
		},
		{
			name:  "drops expired unmanaged ban",
			entry: models.CfgBanEntry{SteamID: "76561198000000001", Expired: true},
			want:  false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := shouldPreserveExistingBansCfgEntry(
				tt.entry,
				tt.managedSteamIDs,
				tt.managedEOSIDs,
				tt.excludedSteamIDs,
				tt.excludedEOSIDs,
			)
			if got != tt.want {
				t.Fatalf("shouldPreserveExistingBansCfgEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildServerBansCfgContent(t *testing.T) {
	t.Parallel()

	expiry := time.Unix(1_893_456_000, 0)
	content := buildServerBansCfgContent([]models.ServerBan{
		{
			AdminName:    "Alice",
			AdminSteamID: "76561198000000010",
			SteamID:      "76561198000000001",
			Reason:       "Cheating",
		},
		{
			EOSID:     "ABCDEF0123456789ABCDEF0123456789",
			ExpiresAt: &expiry,
		},
	})

	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 bans, got %d in %q", len(lines), content)
	}

	if lines[0] != "Alice [SteamID 76561198000000010] Banned:76561198000000001:0 //Cheating" {
		t.Fatalf("unexpected first line: %q", lines[0])
	}

	wantSecondLine := "System [SteamID 0] Banned:abcdef0123456789abcdef0123456789:" + strconv.FormatInt(expiry.Unix(), 10)
	if lines[1] != wantSecondLine {
		t.Fatalf("unexpected second line: %q", lines[1])
	}
}

func TestCollectServerBanIDs(t *testing.T) {
	t.Parallel()

	steamIDs, eosIDs := collectServerBanIDs([]models.ServerBan{
		{SteamID: "76561198000000001"},
		{EOSID: "ABCDEF0123456789ABCDEF0123456789"},
	})

	if !steamIDs["76561198000000001"] {
		t.Fatal("expected steam ID to be collected")
	}

	if !eosIDs["abcdef0123456789abcdef0123456789"] {
		t.Fatal("expected EOS ID to be normalized and collected")
	}
}
