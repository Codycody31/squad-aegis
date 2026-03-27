package rcon_manager

import "testing"

func TestValidateCommandResponseRejectsMismatchedPayloads(t *testing.T) {
	tests := []struct {
		command  string
		response string
	}{
		{
			command:  "ShowServerInfo",
			response: "----- Active Players -----\n----- Recently Disconnected Players [Max of 15] -----",
		},
		{
			command:  "ListPlayers",
			response: `{"ServerName_s":"Test"}`,
		},
		{
			command:  "ListSquads",
			response: "Current level is Gorodok, layer is AAS v1, factions USA RGF",
		},
	}

	for _, tt := range tests {
		if err := validateCommandResponse(tt.command, tt.response); err == nil {
			t.Fatalf("expected validation error for %s", tt.command)
		}
	}
}

func TestValidateCommandResponseAcceptsExpectedPayloads(t *testing.T) {
	tests := []struct {
		command  string
		response string
	}{
		{
			command:  "ShowServerInfo",
			response: `{"ServerName_s":"Test","TeamOne_s":"Blue","TeamTwo_s":"Red"}`,
		},
		{
			command:  "ListPlayers",
			response: "----- Active Players -----\n----- Recently Disconnected Players [Max of 15] -----",
		},
		{
			command:  "ListSquads",
			response: "----- Active Squads -----\nTeam ID: 1 (Blue)\nTeam ID: 2 (Red)",
		},
	}

	for _, tt := range tests {
		if err := validateCommandResponse(tt.command, tt.response); err != nil {
			t.Fatalf("expected %s payload to validate, got error: %v", tt.command, err)
		}
	}
}

func TestDefaultRetriesForCommand(t *testing.T) {
	if retries := defaultRetriesForCommand("ShowServerInfo"); retries != 2 {
		t.Fatalf("expected ShowServerInfo retries to be 2, got %d", retries)
	}

	if retries := defaultRetriesForCommand("ListPlayers"); retries != 2 {
		t.Fatalf("expected ListPlayers retries to be 2, got %d", retries)
	}

	if retries := defaultRetriesForCommand("AdminKick 1 testing"); retries != 1 {
		t.Fatalf("expected AdminKick retries to remain 1, got %d", retries)
	}
}
