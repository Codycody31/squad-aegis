package utils

import "testing"

func TestParseOnlineIDsSupportsEOSSteamAndEpic(t *testing.T) {
	t.Parallel()

	ids := ParseOnlineIDs(" EOS: 0002BB228E4D4363ADA0C139B11A9ECE epic: E91067B2C8BB461EBF0CDF3A01EE5B0B steam: INVALID ")

	if ids.EOSID != "0002bb228e4d4363ada0c139b11a9ece" {
		t.Fatalf("EOSID = %q, want normalized EOS ID", ids.EOSID)
	}
	if ids.EpicID != "e91067b2c8bb461ebf0cdf3a01ee5b0b" {
		t.Fatalf("EpicID = %q, want normalized Epic ID", ids.EpicID)
	}
	if ids.SteamID != "" {
		t.Fatalf("SteamID = %q, want empty string for invalid steam ID", ids.SteamID)
	}
}

func TestParseOnlineIDsIgnoresMissingTokens(t *testing.T) {
	t.Parallel()

	ids := ParseOnlineIDs(" steam: 76561198000000000 ")

	if ids.SteamID != "76561198000000000" {
		t.Fatalf("SteamID = %q, want valid Steam ID", ids.SteamID)
	}
	if ids.EOSID != "" || ids.EpicID != "" {
		t.Fatalf("unexpected non-Steam IDs: %#v", ids)
	}
}
