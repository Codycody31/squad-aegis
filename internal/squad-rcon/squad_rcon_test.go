package squadRcon

import (
	"errors"
	"testing"
)

func TestOrderedTeamNamesFromHeaderOnlyListSquadsResponse(t *testing.T) {
	response := `----- Active Squads -----
Team ID: 0 (Local Civilians that have some limited access to vehicles)
Team ID: 1 (Manticore Security Task Force)
Team ID: 2 (Turkish Land Forces)`

	teamNames := orderedTeamNames(response)
	if len(teamNames) != 2 {
		t.Fatalf("expected 2 team names, got %d", len(teamNames))
	}

	if teamNames[0] != "Manticore Security Task Force" {
		t.Fatalf("expected team 1 name to be preserved, got %q", teamNames[0])
	}

	if teamNames[1] != "Turkish Land Forces" {
		t.Fatalf("expected team 2 name to be preserved, got %q", teamNames[1])
	}
}

func TestParseSquadsResponseToleratesExtendedSquadLines(t *testing.T) {
	response := `----- Active Squads -----
Team ID: 1 (Manticore Security Task Force)
ID: 3 | Name: Armor | Size: 9 | Locked: False | Creator Name: Leader | Creator Online IDs: EOS: 1234567890abcdef1234567890abcdef steam: 76561198000000000
Team ID: 2 (Turkish Land Forces)
ID: 1 | Name: INF | Size: 4/9 | Locked: True`

	squads := parseSquadsResponse(response)
	if len(squads) != 2 {
		t.Fatalf("expected 2 squads, got %d", len(squads))
	}

	if squads[0].TeamId != 1 || squads[0].ID != 3 || squads[0].Name != "Armor" || squads[0].Size != 9 || squads[0].Locked {
		t.Fatalf("unexpected first squad parse result: %+v", squads[0])
	}

	if squads[1].TeamId != 2 || squads[1].ID != 1 || squads[1].Name != "INF" || squads[1].Size != 4 || !squads[1].Locked {
		t.Fatalf("unexpected second squad parse result: %+v", squads[1])
	}
}

func TestParseNextMapResponseHandlesUndefinedNextMap(t *testing.T) {
	_, err := parseNextMapResponse("Next map is not defined")
	if !errors.Is(err, ErrNoNextMap) {
		t.Fatalf("expected ErrNoNextMap, got %v", err)
	}
}

func TestParseNextMapResponseParsesNextMapVariants(t *testing.T) {
	tests := []struct {
		name     string
		response string
	}{
		{
			name:     "level wording",
			response: "Next level is Gorodok, layer is AAS v1, factions USA RGF",
		},
		{
			name:     "map wording",
			response: "Next map is Narva, layer is RAAS v2, factions CAF VDV",
		},
	}

	for _, tt := range tests {
		nextMap, err := parseNextMapResponse(tt.response)
		if err != nil {
			t.Fatalf("%s: expected map to parse, got %v", tt.name, err)
		}

		if nextMap.Map == "" || nextMap.Layer == "" || len(nextMap.Factions) != 2 {
			t.Fatalf("%s: unexpected parse result %+v", tt.name, nextMap)
		}
	}
}

func TestParsePlayersResponseToleratesEpicIdentifiers(t *testing.T) {
	response := `ID: 1 | Online IDs: EOS: ABCDEF0123456789ABCDEF0123456789 steam: 76561198000000000 | Name: Steam User | Team ID: 1 | Squad ID: 2 | Is Leader: False | Role: Rifleman
ID: 2 | Online IDs: EOS: fedcba9876543210fedcba9876543210 epic: 8899aabbccddeeff0011223344556677 steam: INVALID | Name: Epic Placeholder | Team ID: 1 | Squad ID: 3 | Is Leader: True | Role: TL_SL
ID: 3 | Online IDs: EOS: 00112233445566778899AABBCCDDEEFF epic: e91067b2c8bb461ebf0cdf3a01ee5b0b | Name: Epic EOS Only | Team ID: 2 | Squad ID: N/A | Is Leader: False | Role: Rifleman
ID: 4 | Online IDs: EOS: AABBCCDDEEFF00112233445566778899 epic: 11223344556677889900aabbccddeeff steam: INVALID | Since Disconnect: 05m 12s | Name: Left Epic`

	players := parsePlayersResponse(response)

	if got, want := len(players.OnlinePlayers), 3; got != want {
		t.Fatalf("online player count = %d, want %d", got, want)
	}
	if got, want := len(players.DisconnectedPlayers), 1; got != want {
		t.Fatalf("disconnected player count = %d, want %d", got, want)
	}

	steamAndEOS := players.OnlinePlayers[0]
	if steamAndEOS.SteamId != "76561198000000000" {
		t.Fatalf("steam user steam ID = %q, want valid steam ID", steamAndEOS.SteamId)
	}
	if steamAndEOS.EosId != "abcdef0123456789abcdef0123456789" {
		t.Fatalf("steam user EOS ID = %q, want normalized EOS ID", steamAndEOS.EosId)
	}

	epicWithInvalidSteam := players.OnlinePlayers[1]
	if epicWithInvalidSteam.SteamId != "" {
		t.Fatalf("epic placeholder steam ID = %q, want empty string", epicWithInvalidSteam.SteamId)
	}
	if epicWithInvalidSteam.EpicId != "8899aabbccddeeff0011223344556677" {
		t.Fatalf("epic placeholder epic ID = %q, want normalized Epic ID", epicWithInvalidSteam.EpicId)
	}
	if !epicWithInvalidSteam.IsSquadLeader {
		t.Fatalf("expected epic placeholder player to remain squad leader")
	}
	if epicWithInvalidSteam.SquadId != 3 {
		t.Fatalf("epic placeholder squad ID = %d, want 3", epicWithInvalidSteam.SquadId)
	}

	epicEOSOnly := players.OnlinePlayers[2]
	if epicEOSOnly.SteamId != "" {
		t.Fatalf("epic EOS-only steam ID = %q, want empty string", epicEOSOnly.SteamId)
	}
	if epicEOSOnly.EosId != "00112233445566778899aabbccddeeff" {
		t.Fatalf("epic EOS-only EOS ID = %q, want normalized EOS ID", epicEOSOnly.EosId)
	}
	if epicEOSOnly.EpicId != "e91067b2c8bb461ebf0cdf3a01ee5b0b" {
		t.Fatalf("epic EOS-only epic ID = %q, want normalized Epic ID", epicEOSOnly.EpicId)
	}
	if epicEOSOnly.SquadId != 0 {
		t.Fatalf("epic EOS-only squad ID = %d, want 0 for N/A", epicEOSOnly.SquadId)
	}

	disconnectedEpic := players.DisconnectedPlayers[0]
	if disconnectedEpic.SteamId != "" {
		t.Fatalf("disconnected epic steam ID = %q, want empty string", disconnectedEpic.SteamId)
	}
	if disconnectedEpic.EosId != "aabbccddeeff00112233445566778899" {
		t.Fatalf("disconnected epic EOS ID = %q, want normalized EOS ID", disconnectedEpic.EosId)
	}
	if disconnectedEpic.EpicId != "11223344556677889900aabbccddeeff" {
		t.Fatalf("disconnected epic epic ID = %q, want normalized Epic ID", disconnectedEpic.EpicId)
	}
	if disconnectedEpic.SinceDisconnect != "05m12s" {
		t.Fatalf("disconnected epic time = %q, want %q", disconnectedEpic.SinceDisconnect, "05m12s")
	}
}

func TestParseTeamsAndSquadsIncludesEOSOnlyPlayers(t *testing.T) {
	squads := []Squad{
		{ID: 3, TeamId: 1, Name: "Armor"},
		{ID: 1, TeamId: 2, Name: "Inf"},
	}
	teamNames := []string{"Blue", "Red"}
	players := PlayersData{
		OnlinePlayers: []Player{
			{
				Id:            1,
				EosId:         "abcdef0123456789abcdef0123456789",
				Name:          "Epic Squad Leader",
				TeamId:        1,
				SquadId:       3,
				IsSquadLeader: true,
				Role:          "TL_SL",
			},
			{
				Id:      2,
				SteamId: "76561198000000000",
				Name:    "Steam Squadmate",
				TeamId:  1,
				SquadId: 3,
				Role:    "Rifleman",
			},
			{
				Id:      3,
				EosId:   "00112233445566778899aabbccddeeff",
				Name:    "Epic Unassigned",
				TeamId:  2,
				SquadId: 0,
				Role:    "Medic",
			},
		},
	}

	teams, err := ParseTeamsAndSquads(squads, teamNames, players)
	if err != nil {
		t.Fatalf("ParseTeamsAndSquads() returned error: %v", err)
	}

	if got, want := len(teams), 2; got != want {
		t.Fatalf("team count = %d, want %d", got, want)
	}

	if got, want := len(teams[0].Squads), 1; got != want {
		t.Fatalf("team 1 squad count = %d, want %d", got, want)
	}
	if got, want := len(teams[0].Squads[0].Players), 2; got != want {
		t.Fatalf("team 1 squad player count = %d, want %d", got, want)
	}
	if teams[0].Squads[0].Leader == nil || teams[0].Squads[0].Leader.Name != "Epic Squad Leader" {
		t.Fatalf("expected EOS-only squad leader to be preserved, got %+v", teams[0].Squads[0].Leader)
	}

	if got, want := len(teams[1].Players), 1; got != want {
		t.Fatalf("team 2 unassigned player count = %d, want %d", got, want)
	}
	if teams[1].Players[0].Name != "Epic Unassigned" {
		t.Fatalf("unexpected team 2 unassigned player: %+v", teams[1].Players[0])
	}
}
