package server

import "testing"

func TestValidateSquadGamePath(t *testing.T) {
	valid := "/home/squad/serverfiles/SquadGame"
	if err := validateSquadGamePath(&valid); err != nil {
		t.Fatalf("expected valid path, got error: %v", err)
	}

	tests := []string{
		"",
		"   ",
		"/home/squad/serverfiles/SquadGame/Saved/Logs/SquadGame.log",
		"/home/squad/serverfiles/SquadGame/ServerConfig/Bans.cfg",
		"/home/squad/serverfiles/SquadGame/ServerConfig",
	}

	for _, value := range tests {
		value := value
		if err := validateSquadGamePath(&value); err == nil {
			t.Fatalf("expected error for path %q", value)
		}
	}
}
