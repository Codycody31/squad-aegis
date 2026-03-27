package plugin_manager

import "testing"

func TestHasMultipleSQLStatements(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{
			name:  "allows single select without semicolon",
			query: "SELECT 1",
			want:  false,
		},
		{
			name:  "allows trailing semicolon",
			query: "SELECT 1;",
			want:  false,
		},
		{
			name:  "allows trailing semicolon with comment",
			query: "SELECT 1; -- keep this query readable",
			want:  false,
		},
		{
			name:  "allows semicolon inside string literal",
			query: "SELECT ';' AS separator;",
			want:  false,
		},
		{
			name:  "allows semicolon inside quoted identifier",
			query: `SELECT "semi;colon" FROM "table";`,
			want:  false,
		},
		{
			name:  "allows semicolon inside block comment",
			query: "SELECT 1 /* ; */;",
			want:  false,
		},
		{
			name:  "allows semicolon inside dollar-quoted string",
			query: "SELECT $$;$$;",
			want:  false,
		},
		{
			name:  "rejects second statement",
			query: "SELECT 1; SELECT 2",
			want:  true,
		},
		{
			name:  "rejects second statement after comment",
			query: "SELECT 1; -- done\nSELECT 2",
			want:  true,
		},
		{
			name:  "rejects duplicate top-level semicolons",
			query: "SELECT 1;;",
			want:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := hasMultipleSQLStatements(tt.query); got != tt.want {
				t.Fatalf("hasMultipleSQLStatements(%q) = %v, want %v", tt.query, got, tt.want)
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
