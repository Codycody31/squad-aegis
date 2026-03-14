package server

import (
	"testing"
	"time"

	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
)

func TestParseBansCfg_FullFormat(t *testing.T) {
	content := `Admin [SteamID 76561198000000001] Banned:76561198000000002:0 //Cheating
System [SteamID 0] Banned:76561198000000003:1893456000 //Teamkilling`

	entries, unparseable := parseBansCfg(content)

	if unparseable != 0 {
		t.Fatalf("expected 0 unparseable, got %d", unparseable)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// First entry: permanent ban
	if entries[0].SteamID != "76561198000000002" {
		t.Errorf("expected steam ID 76561198000000002, got %s", entries[0].SteamID)
	}
	if !entries[0].Permanent {
		t.Error("expected permanent ban")
	}
	if entries[0].Reason != "Cheating" {
		t.Errorf("expected reason 'Cheating', got %q", entries[0].Reason)
	}

	// Second entry: timed ban (future)
	if entries[1].SteamID != "76561198000000003" {
		t.Errorf("expected steam ID 76561198000000003, got %s", entries[1].SteamID)
	}
	if entries[1].Permanent {
		t.Error("expected non-permanent ban")
	}
	if entries[1].ExpiryTimestamp != 1893456000 {
		t.Errorf("expected expiry 1893456000, got %d", entries[1].ExpiryTimestamp)
	}
}

func TestParseBansCfg_ExpiredBans(t *testing.T) {
	content := "Admin [SteamID 0] Banned:76561198000000001:1000000000 //Old ban"

	entries, unparseable := parseBansCfg(content)

	if unparseable != 0 {
		t.Fatalf("expected 0 unparseable, got %d", unparseable)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if !entries[0].Expired {
		t.Error("expected ban to be marked as expired")
	}
}

func TestParseBansCfg_EmptyAndComments(t *testing.T) {
	content := `
# This is a comment

# Another comment
`

	entries, unparseable := parseBansCfg(content)

	if unparseable != 0 {
		t.Fatalf("expected 0 unparseable, got %d", unparseable)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseBansCfg_UnparseableLines(t *testing.T) {
	content := `This is not a valid ban line
Admin [SteamID 0] Banned:76561198000000001:0 //Valid
Another invalid line
Also invalid: no banned prefix`

	entries, unparseable := parseBansCfg(content)

	if unparseable != 3 {
		t.Fatalf("expected 3 unparseable, got %d", unparseable)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestParseBansCfg_DuplicateSteamIDs(t *testing.T) {
	content := `Admin [SteamID 0] Banned:76561198000000001:0 //First ban
Admin [SteamID 0] Banned:76561198000000001:0 //Duplicate ban`

	entries, _ := parseBansCfg(content)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (deduplicated), got %d", len(entries))
	}
	if entries[0].Reason != "First ban" {
		t.Errorf("expected first occurrence to win, got reason %q", entries[0].Reason)
	}
}

func TestParseBansCfg_NoReason(t *testing.T) {
	content := `Admin [SteamID 0] Banned:76561198000000001:0`

	entries, unparseable := parseBansCfg(content)

	if unparseable != 0 {
		t.Fatalf("expected 0 unparseable, got %d", unparseable)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Reason != "" {
		t.Errorf("expected empty reason, got %q", entries[0].Reason)
	}
}

func TestParseBansCfg_InvalidSteamID(t *testing.T) {
	// "notanumber" is neither a 32-char hex EOS ID nor a numeric Steam ID
	content := `Admin [SteamID 0] Banned:notanumber:0 //Bad ID`

	entries, unparseable := parseBansCfg(content)

	if unparseable != 1 {
		t.Fatalf("expected 1 unparseable, got %d", unparseable)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseBansCfg_InvalidExpiry(t *testing.T) {
	content := `Admin [SteamID 0] Banned:76561198000000001:notanumber //Bad expiry`

	entries, unparseable := parseBansCfg(content)

	if unparseable != 1 {
		t.Fatalf("expected 1 unparseable, got %d", unparseable)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseBansCfg_MixedContent(t *testing.T) {
	content := "# Ban list\nAdmin [SteamID 76561198000000001] Banned:76561198000000010:0 //Permanent cheater\n\nSystem [SteamID 0] Banned:76561198000000011:9999999999 //Future temp ban\ngarbage line\nAdmin [SteamID 0] Banned:76561198000000012:1000000000 //Old expired ban\n"

	entries, unparseable := parseBansCfg(content)

	if unparseable != 1 {
		t.Fatalf("expected 1 unparseable, got %d", unparseable)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Check permanent ban (expiry = 0)
	if !entries[0].Permanent {
		t.Error("first entry should be permanent")
	}

	// Check 9999999999 — should be treated as permanent (threshold-based)
	if !entries[1].Permanent {
		t.Error("second entry (9999999999) should be treated as permanent")
	}
	if entries[1].Expired {
		t.Error("second entry should not be expired")
	}

	// Check expired ban
	if !entries[2].Expired {
		t.Error("third entry should be expired")
	}
}

func TestParseBansCfg_ReasonWithColons(t *testing.T) {
	content := `Admin [SteamID 0] Banned:76561198000000001:0 //Reason: with colons: in it`

	entries, unparseable := parseBansCfg(content)

	if unparseable != 0 {
		t.Fatalf("expected 0 unparseable, got %d", unparseable)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Reason != "Reason: with colons: in it" {
		t.Errorf("expected reason with colons preserved, got %q", entries[0].Reason)
	}
}

func TestParseBansCfg_EmptyFile(t *testing.T) {
	entries, unparseable := parseBansCfg("")

	if unparseable != 0 {
		t.Fatalf("expected 0 unparseable, got %d", unparseable)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseBansCfg_EOSID(t *testing.T) {
	content := `N/A Banned:0002adb8a89b4d1d970a3cd1e4569092:10403758725 //Griefing
N/A Banned:0002c835e7db4415b9f823b95b5b90b6:1765307819 //Spawn camping`

	entries, unparseable := parseBansCfg(content)

	if unparseable != 0 {
		t.Fatalf("expected 0 unparseable, got %d", unparseable)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// First entry: EOS ID, far-future expiry (treated as permanent)
	if entries[0].EOSID != "0002adb8a89b4d1d970a3cd1e4569092" {
		t.Errorf("expected EOS ID 0002adb8a89b4d1d970a3cd1e4569092, got %s", entries[0].EOSID)
	}
	if entries[0].SteamID != "" {
		t.Errorf("expected empty steam ID for EOS entry, got %s", entries[0].SteamID)
	}
	if !entries[0].Permanent {
		t.Error("expected EOS ban with 10403758725 expiry to be treated as permanent")
	}

	// Second entry: EOS ID, future timed ban
	if entries[1].EOSID != "0002c835e7db4415b9f823b95b5b90b6" {
		t.Errorf("expected EOS ID 0002c835e7db4415b9f823b95b5b90b6, got %s", entries[1].EOSID)
	}
}

func TestParseBansCfg_PermanentThreshold(t *testing.T) {
	// Test various permanent representations
	content := `Admin [SteamID 0] Banned:76561198000000001:0 //Permanent via zero
Admin [SteamID 0] Banned:76561198000000002:9999999999 //Permanent via far future`

	entries, unparseable := parseBansCfg(content)

	if unparseable != 0 {
		t.Fatalf("expected 0 unparseable, got %d", unparseable)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if !entries[0].Permanent {
		t.Error("expiry=0 should be permanent")
	}
	if !entries[1].Permanent {
		t.Error("expiry=9999999999 should be treated as permanent")
	}
	if entries[0].Expired || entries[1].Expired {
		t.Error("permanent bans should not be marked as expired")
	}
}

func TestParseBansCfg_AutoBanDetection(t *testing.T) {
	content := `N/A Banned:0002c6fc68c04dad8ad44cb9c83b2187:1766283597 //Automatic Teamkill Kick
Admin [SteamID 0] Banned:76561198000000001:0 //Manual ban
N/A Banned:76561199857143702:1758370309 //Automatic Server Rule Violation`

	entries, unparseable := parseBansCfg(content)

	if unparseable != 0 {
		t.Fatalf("expected 0 unparseable, got %d", unparseable)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if !entries[0].IsAutoBan {
		t.Error("'Automatic Teamkill Kick' should be classified as auto-ban")
	}
	if entries[1].IsAutoBan {
		t.Error("manual ban should NOT be classified as auto-ban")
	}
	if !entries[2].IsAutoBan {
		t.Error("'Automatic Server Rule Violation' should be classified as auto-ban")
	}
}

func TestParseBansCfg_MixedSteamAndEOS(t *testing.T) {
	// Real-world Bans.cfg with mixed identifier types
	content := `insidiousfiddler [SteamID 76561199047801300] Banned:76561199814503607:1763087080 //2.3 | No toxicity/harassment. Help new players | 1 day
insidiousfiddler [SteamID 76561199047801300] Banned:76561199817970666:9999999999 //Community Health, racism
envixity [SteamID 76561199151514762] Banned:76561199857143702:1758370309 //Griefing / Trolling | Length: 7 days
N/A Banned:0002adb8a89b4d1d970a3cd1e4569092:10403758725 //Griefing
N/A Banned:0002c835e7db4415b9f823b95b5b90b6:1765307819 //Spawn camping
N/A Banned:0002c6fc68c04dad8ad44cb9c83b2187:1766283597 //Automatic Teamkill Kick`

	entries, unparseable := parseBansCfg(content)

	if unparseable != 0 {
		t.Fatalf("expected 0 unparseable, got %d", unparseable)
	}
	if len(entries) != 6 {
		t.Fatalf("expected 6 entries, got %d", len(entries))
	}

	// Steam ID entries
	if entries[0].SteamID != "76561199814503607" || entries[0].EOSID != "" {
		t.Errorf("entry 0: expected Steam ID only, got steam=%q eos=%q", entries[0].SteamID, entries[0].EOSID)
	}

	// 9999999999 = permanent
	if !entries[1].Permanent {
		t.Error("entry 1: 9999999999 should be permanent")
	}

	// EOS entries
	if entries[3].EOSID != "0002adb8a89b4d1d970a3cd1e4569092" || entries[3].SteamID != "" {
		t.Errorf("entry 3: expected EOS ID only, got steam=%q eos=%q", entries[3].SteamID, entries[3].EOSID)
	}

	// Auto-ban
	if !entries[5].IsAutoBan {
		t.Error("entry 5: should be classified as auto-ban")
	}
	if entries[4].IsAutoBan {
		t.Error("entry 4: 'Spawn camping' should NOT be auto-ban")
	}
}

func TestParseBansCfg_DuplicateEOSIDs(t *testing.T) {
	content := `N/A Banned:0002adb8a89b4d1d970a3cd1e4569092:0 //First
N/A Banned:0002adb8a89b4d1d970a3cd1e4569092:0 //Duplicate`

	entries, _ := parseBansCfg(content)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (deduplicated), got %d", len(entries))
	}
	if entries[0].Reason != "First" {
		t.Errorf("expected first occurrence to win, got reason %q", entries[0].Reason)
	}
}

func TestParseBansCfg_NAPrefixFormat(t *testing.T) {
	// "N/A" prefix is common for server-generated bans
	content := `N/A Banned:76561199857143702:1758370309 //Griefing / Trolling | Length: 7 days`

	entries, unparseable := parseBansCfg(content)

	if unparseable != 0 {
		t.Fatalf("expected 0 unparseable, got %d", unparseable)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].SteamID != "76561199857143702" {
		t.Errorf("expected steam ID 76561199857143702, got %s", entries[0].SteamID)
	}
}

func TestParseBansCfg_ReasonWithPipe(t *testing.T) {
	// Reasons often contain pipe-separated metadata
	content := `admin [SteamID 76561199047801300] Banned:76561199814503607:1763087080 //2.3 | No toxicity/harassment. Help new players | 1 day`

	entries, unparseable := parseBansCfg(content)

	if unparseable != 0 {
		t.Fatalf("expected 0 unparseable, got %d", unparseable)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Reason != "2.3 | No toxicity/harassment. Help new players | 1 day" {
		t.Errorf("expected full reason with pipes, got %q", entries[0].Reason)
	}
}

func TestIsHex(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"0002adb8a89b4d1d970a3cd1e4569092", true},
		{"abcdef0123456789ABCDEF0123456789", true},
		{"0000000000000000000000000000000g", false}, // 'g' is not hex
		{"", false},
		{"76561199814503607", false}, // numeric but not 32 chars (handled separately in parser)
	}

	for _, tt := range tests {
		result := utils.IsHex(tt.input)
		if result != tt.expected {
			t.Errorf("isHex(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestCategorizeBans(t *testing.T) {
	now := time.Now()
	futureExpiry := now.Add(30 * 24 * time.Hour).Unix()

	entries := []models.CfgBanEntry{
		{SteamID: "1001", Reason: "new steam ban", Permanent: true},
		{SteamID: "1002", Reason: "existing steam ban", Permanent: true},
		{EOSID: "aabbccdd11223344aabbccdd11223344", Reason: "new eos ban", ExpiryTimestamp: futureExpiry},
		{EOSID: "eeff00112233445566778899aabbccdd", Reason: "existing eos ban", Permanent: true},
		{SteamID: "1003", Reason: "expired", Expired: true},
		{SteamID: "1004", Reason: "Automatic Teamkill Kick", IsAutoBan: true},
	}

	existingSteamIDs := map[string]bool{"1002": true}
	existingEOSIDs := map[string]bool{"eeff00112233445566778899aabbccdd": true}

	newBans, existingBans, expiredBans, autoBans := categorizeBans(entries, existingSteamIDs, existingEOSIDs)

	if len(newBans) != 2 {
		t.Errorf("expected 2 new bans, got %d", len(newBans))
	}
	if len(existingBans) != 2 {
		t.Errorf("expected 2 existing bans, got %d", len(existingBans))
	}
	if len(expiredBans) != 1 {
		t.Errorf("expected 1 expired ban, got %d", len(expiredBans))
	}
	if len(autoBans) != 1 {
		t.Errorf("expected 1 auto-ban, got %d", len(autoBans))
	}
}

func TestParseBansCfg_NormalizesEOSIDCase(t *testing.T) {
	content := `N/A Banned:ABCDEF0123456789ABCDEF0123456789:0 //Uppercase EOS`

	entries, unparseable := parseBansCfg(content)
	if unparseable != 0 {
		t.Fatalf("expected 0 unparseable, got %d", unparseable)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].EOSID != "abcdef0123456789abcdef0123456789" {
		t.Fatalf("expected normalized lowercase EOS ID, got %q", entries[0].EOSID)
	}
}

func TestCalculateImportedBanTimingPreservesExpiry(t *testing.T) {
	now := time.Date(2026, time.March, 14, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		expiry   time.Time
		wantDays int
	}{
		{
			name:     "less than one day remaining rounds up to one day",
			expiry:   now.Add(2 * time.Hour),
			wantDays: 1,
		},
		{
			name:     "more than one day remaining preserves original expiry",
			expiry:   now.Add(25 * time.Hour),
			wantDays: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			durationDays, createdAt := calculateImportedBanTiming(tt.expiry.Unix(), now)
			if durationDays != tt.wantDays {
				t.Fatalf("durationDays = %d, want %d", durationDays, tt.wantDays)
			}

			regeneratedExpiry := createdAt.AddDate(0, 0, durationDays)
			if regeneratedExpiry.Unix() != tt.expiry.Unix() {
				t.Fatalf("regenerated expiry = %d, want %d", regeneratedExpiry.Unix(), tt.expiry.Unix())
			}
		})
	}
}
