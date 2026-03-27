package utils

import "testing"

func TestNormalizePlayerIdentifiers(t *testing.T) {
	const steamID = "76561198000000042"
	const eosID = "ABCDEF0123456789ABCDEF0123456789"

	tests := []struct {
		name     string
		playerID string
		steamID  string
		eosID    string
		want     PlayerIdentifiers
	}{
		{
			name:     "steam only",
			playerID: steamID,
			steamID:  steamID,
			want: PlayerIdentifiers{
				PlayerID: steamID,
				SteamID:  steamID,
			},
		},
		{
			name:     "eos only",
			playerID: eosID,
			eosID:    eosID,
			want: PlayerIdentifiers{
				PlayerID: NormalizeEOSID(eosID),
				EOSID:    NormalizeEOSID(eosID),
			},
		},
		{
			name:     "dual ids prefer steam",
			playerID: eosID,
			steamID:  steamID,
			eosID:    eosID,
			want: PlayerIdentifiers{
				PlayerID: steamID,
				SteamID:  steamID,
				EOSID:    NormalizeEOSID(eosID),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePlayerIdentifiers(tt.playerID, tt.steamID, tt.eosID)
			if got != tt.want {
				t.Fatalf("NormalizePlayerIdentifiers() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPlayerIdentifiersStorageIDs(t *testing.T) {
	ids := NormalizePlayerIdentifiers("abcdef0123456789abcdef0123456789", "76561198000000042", "abcdef0123456789abcdef0123456789")
	got := ids.StorageIDs()
	want := []string{"76561198000000042", "abcdef0123456789abcdef0123456789"}

	if len(got) != len(want) {
		t.Fatalf("len(StorageIDs()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("StorageIDs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMergePlayerIdentifiersPreservesExistingAliasesOnPartialUpdate(t *testing.T) {
	existing := NormalizePlayerIdentifiers("", "76561198000000042", "abcdef0123456789abcdef0123456789")
	incoming := NormalizePlayerIdentifiers("", "76561198000000042", "")

	got := MergePlayerIdentifiers(existing, incoming)

	want := PlayerIdentifiers{
		PlayerID: "76561198000000042",
		SteamID:  "76561198000000042",
		EOSID:    "abcdef0123456789abcdef0123456789",
	}
	if got != want {
		t.Fatalf("MergePlayerIdentifiers() = %+v, want %+v", got, want)
	}
}

func TestPlayerIdentifiersIncludesAll(t *testing.T) {
	tests := []struct {
		name     string
		resolved PlayerIdentifiers
		request  PlayerIdentifiers
		want     bool
	}{
		{
			name: "matched steam and eos pair",
			resolved: NormalizePlayerIdentifiers(
				"",
				"76561198000000042",
				"abcdef0123456789abcdef0123456789",
			),
			request: NormalizePlayerIdentifiers(
				"",
				"76561198000000042",
				"ABCDEF0123456789ABCDEF0123456789",
			),
			want: true,
		},
		{
			name: "mismatched eos is rejected even when another eos exists",
			resolved: NormalizePlayerIdentifiers(
				"",
				"76561198000000042",
				"abcdef0123456789abcdef0123456789",
			),
			request: NormalizePlayerIdentifiers(
				"",
				"76561198000000042",
				"ffffffffffffffffffffffffffffffff",
			),
			want: false,
		},
		{
			name: "single identifier only needs to match itself",
			resolved: NormalizePlayerIdentifiers(
				"",
				"76561198000000042",
				"abcdef0123456789abcdef0123456789",
			),
			request: NormalizePlayerIdentifiers("", "", "abcdef0123456789abcdef0123456789"),
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resolved.IncludesAll(tt.request); got != tt.want {
				t.Fatalf("IncludesAll() = %v, want %v", got, tt.want)
			}
		})
	}
}
