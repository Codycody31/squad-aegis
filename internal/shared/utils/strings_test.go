package utils

import "testing"

func TestNormalizeEOSID(t *testing.T) {
	input := "  ABCDEF0123456789ABCDEF0123456789  "
	got := NormalizeEOSID(input)
	want := "abcdef0123456789abcdef0123456789"

	if got != want {
		t.Fatalf("NormalizeEOSID(%q) = %q, want %q", input, got, want)
	}

	if !IsEOSID(got) {
		t.Fatalf("normalized EOS ID %q should validate", got)
	}
}

func TestNormalizePlayerID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "steam", input: " 76561198000000001 ", want: "76561198000000001"},
		{name: "eos", input: " ABCDEF0123456789ABCDEF0123456789 ", want: "abcdef0123456789abcdef0123456789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizePlayerID(tt.input); got != tt.want {
				t.Fatalf("NormalizePlayerID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchPlayerID(t *testing.T) {
	if !MatchPlayerID("abcdef0123456789abcdef0123456789", "", "ABCDEF0123456789ABCDEF0123456789") {
		t.Fatalf("expected EOS IDs to match after normalization")
	}

	if !MatchPlayerID("76561198000000001", "76561198000000001", "") {
		t.Fatalf("expected Steam IDs to match")
	}

	if MatchPlayerID("76561198000000001", "", "abcdef0123456789abcdef0123456789") {
		t.Fatalf("unexpected cross-identifier match")
	}
}

func TestParsePlayerID(t *testing.T) {
	steamID, eosID, normalized, err := ParsePlayerID("76561198000000001")
	if err != nil {
		t.Fatalf("ParsePlayerID(steam) returned error: %v", err)
	}
	if steamID == nil || *steamID != 76561198000000001 {
		t.Fatalf("expected parsed Steam ID, got %v", steamID)
	}
	if eosID != nil {
		t.Fatalf("expected nil EOS ID, got %v", *eosID)
	}
	if normalized != "76561198000000001" {
		t.Fatalf("unexpected normalized Steam ID: %q", normalized)
	}

	steamID, eosID, normalized, err = ParsePlayerID("ABCDEF0123456789ABCDEF0123456789")
	if err != nil {
		t.Fatalf("ParsePlayerID(eos) returned error: %v", err)
	}
	if steamID != nil {
		t.Fatalf("expected nil Steam ID, got %v", *steamID)
	}
	if eosID == nil || *eosID != "abcdef0123456789abcdef0123456789" {
		t.Fatalf("expected normalized EOS ID, got %v", eosID)
	}
	if normalized != "abcdef0123456789abcdef0123456789" {
		t.Fatalf("unexpected normalized EOS ID: %q", normalized)
	}
}
