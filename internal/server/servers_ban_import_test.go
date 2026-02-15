package server

import (
	"testing"
	"time"
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

	// Second entry: timed ban
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
	// Use a past timestamp
	pastTimestamp := time.Now().Add(-24 * time.Hour).Unix()
	content := `Admin [SteamID 0] Banned:76561198000000001:` + time.Unix(pastTimestamp, 0).Format("") + ` //Expired`

	// Actually use a known past timestamp directly
	content = "Admin [SteamID 0] Banned:76561198000000001:1000000000 //Old ban"

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
	futureTimestamp := time.Now().Add(30 * 24 * time.Hour).Unix()
	content := `# Ban list
Admin [SteamID 76561198000000001] Banned:76561198000000010:0 //Permanent cheater

System [SteamID 0] Banned:76561198000000011:` + time.Unix(futureTimestamp, 0).Format("") + ` //Temp ban
garbage line
Admin [SteamID 0] Banned:76561198000000012:1000000000 //Old expired ban
`
	// Fix: use a proper future timestamp
	content = "# Ban list\nAdmin [SteamID 76561198000000001] Banned:76561198000000010:0 //Permanent cheater\n\nSystem [SteamID 0] Banned:76561198000000011:9999999999 //Future temp ban\ngarbage line\nAdmin [SteamID 0] Banned:76561198000000012:1000000000 //Old expired ban\n"

	entries, unparseable := parseBansCfg(content)

	if unparseable != 1 {
		t.Fatalf("expected 1 unparseable, got %d", unparseable)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Check permanent ban
	if !entries[0].Permanent {
		t.Error("first entry should be permanent")
	}

	// Check future temp ban
	if entries[1].Permanent || entries[1].Expired {
		t.Error("second entry should be non-permanent, non-expired")
	}

	// Check expired ban
	if !entries[2].Expired {
		t.Error("third entry should be expired")
	}
}

func TestParseBansCfg_ReasonWithColons(t *testing.T) {
	// The reason may contain colons after the "//" prefix
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
