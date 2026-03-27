package squadRcon

import "testing"

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
