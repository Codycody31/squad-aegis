package server

import (
	"fmt"
	"reflect"
	"testing"
)

func TestMergeResolvedPlayerIdentifiersPrefersSteam(t *testing.T) {
	t.Parallel()

	identifiers := mergeResolvedPlayerIdentifiers(
		[]string{"76561198000000001"},
		[]string{"ABCDEF0123456789ABCDEF0123456789"},
	)

	if identifiers.PlayerID != "76561198000000001" {
		t.Fatalf("expected Steam ID to be canonical, got %q", identifiers.PlayerID)
	}
	if identifiers.EOSID != "abcdef0123456789abcdef0123456789" {
		t.Fatalf("expected normalized EOS ID, got %q", identifiers.EOSID)
	}
}

func TestMergeVerifiedPlayerIdentifiersIgnoresUnlinkedAlias(t *testing.T) {
	t.Parallel()

	identifiers := mergeVerifiedPlayerIdentifiers(
		mergeResolvedPlayerIdentifiers(
			[]string{"76561198000000001"},
			[]string{"ABCDEF0123456789ABCDEF0123456789"},
		),
		nil,
		nil,
	)

	if identifiers.PlayerID != "76561198000000001" {
		t.Fatalf("expected Steam ID to stay canonical, got %q", identifiers.PlayerID)
	}
	if identifiers.SteamID != "76561198000000001" {
		t.Fatalf("expected Steam ID to be preserved, got %q", identifiers.SteamID)
	}
	if identifiers.EOSID != "" {
		t.Fatalf("expected unverified EOS ID to be dropped, got %q", identifiers.EOSID)
	}
}

func TestMergeVerifiedPlayerIdentifiersIncludesLinkedAlias(t *testing.T) {
	t.Parallel()

	identifiers := mergeVerifiedPlayerIdentifiers(
		mergeResolvedPlayerIdentifiers(
			[]string{"76561198000000001"},
			[]string{"ABCDEF0123456789ABCDEF0123456789"},
		),
		[]string{"76561198000000001"},
		[]string{"ABCDEF0123456789ABCDEF0123456789"},
	)

	if identifiers.EOSID != "abcdef0123456789abcdef0123456789" {
		t.Fatalf("expected linked EOS ID to be preserved, got %q", identifiers.EOSID)
	}
}

func TestBuildPlayerRuleViolationWhereClauseIncludesCanonicalAndAliases(t *testing.T) {
	t.Parallel()

	whereClause, args := buildPlayerRuleViolationWhereClause(
		[]string{"76561198000000001"},
		[]string{"ABCDEF0123456789ABCDEF0123456789"},
		"pv.",
	)

	wantClause := "pv.player_id = ? OR pv.player_id = ? OR pv.player_steam_id = ? OR pv.player_eos_id = ?"
	if whereClause != wantClause {
		t.Fatalf("unexpected where clause:\n got: %q\nwant: %q", whereClause, wantClause)
	}

	wantArgs := []interface{}{
		"76561198000000001",
		"abcdef0123456789abcdef0123456789",
		int64(76561198000000001),
		"abcdef0123456789abcdef0123456789",
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args:\n got: %#v\nwant: %#v", args, wantArgs)
	}
}

func TestParseSteamIdentifierListSkipsInvalidAndDeduplicates(t *testing.T) {
	t.Parallel()

	got := parseSteamIdentifierList([]string{
		"76561198000000001",
		"not-a-steam-id",
		"76561198000000001",
		"76561198000000002",
	})

	want := []uint64{76561198000000001, 76561198000000002}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected parsed Steam IDs:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestBuildClickHouseIdentifierWhereClause(t *testing.T) {
	t.Parallel()

	whereClause, args := buildClickHouseIdentifierWhereClause(
		"steam_id",
		"eos_id",
		[]string{"76561198000000001"},
		[]string{"abcdef0123456789abcdef0123456789"},
		true,
	)

	wantClause := "steam_id IN (?) OR eos_id IN (?)"
	if whereClause != wantClause {
		t.Fatalf("unexpected where clause:\n got: %q\nwant: %q", whereClause, wantClause)
	}

	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}

	steamArgs, ok := args[0].([]uint64)
	if !ok || !reflect.DeepEqual(steamArgs, []uint64{76561198000000001}) {
		t.Fatalf("unexpected steam args: %#v", args[0])
	}

	eosArgs, ok := args[1].([]string)
	if !ok || !reflect.DeepEqual(eosArgs, []string{"abcdef0123456789abcdef0123456789"}) {
		t.Fatalf("unexpected EOS args: %#v", args[1])
	}
}

func TestBuildClickHouseIdentifierWhereClauseSupportsPlayerHistoryColumns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		steamColumn string
		eosColumn   string
	}{
		{
			name:        "recent servers columns",
			steamColumn: "steam",
			eosColumn:   "eos",
		},
		{
			name:        "recent activity death columns",
			steamColumn: "victim_steam",
			eosColumn:   "victim_eos",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			whereClause, args := buildClickHouseIdentifierWhereClause(
				tt.steamColumn,
				tt.eosColumn,
				[]string{"76561198000000001"},
				[]string{"abcdef0123456789abcdef0123456789"},
				false,
			)

			wantClause := fmt.Sprintf("%s IN (?) OR %s IN (?)", tt.steamColumn, tt.eosColumn)
			if whereClause != wantClause {
				t.Fatalf("unexpected where clause:\n got: %q\nwant: %q", whereClause, wantClause)
			}

			wantArgs := []interface{}{
				[]string{"76561198000000001"},
				[]string{"abcdef0123456789abcdef0123456789"},
			}
			if !reflect.DeepEqual(args, wantArgs) {
				t.Fatalf("unexpected args:\n got: %#v\nwant: %#v", args, wantArgs)
			}
		})
	}
}
