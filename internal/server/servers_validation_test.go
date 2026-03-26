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
		"/home/squad/../../etc",
		"/home/squad/serverfiles/../../../etc/passwd",
		"../relative/path",
	}

	for _, value := range tests {
		value := value
		if err := validateSquadGamePath(&value); err == nil {
			t.Fatalf("expected error for path %q", value)
		}
	}
}

func TestNormalizeLogSourceType(t *testing.T) {
	t.Parallel()

	local := "local"
	blank := "  "
	sftp := " sftp "
	invalid := "scp"

	tests := []struct {
		name    string
		input   *string
		want    *string
		wantErr bool
	}{
		{name: "nil stays nil", input: nil, want: nil},
		{name: "blank becomes nil", input: &blank, want: nil},
		{name: "local is accepted", input: &local, want: &local},
		{name: "trimmed sftp is accepted", input: &sftp, want: stringPtr("sftp")},
		{name: "invalid is rejected", input: &invalid, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeLogSourceType(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			switch {
			case tt.want == nil && got != nil:
				t.Fatalf("expected nil, got %q", *got)
			case tt.want != nil && got == nil:
				t.Fatalf("expected %q, got nil", *tt.want)
			case tt.want != nil && got != nil && *tt.want != *got:
				t.Fatalf("expected %q, got %q", *tt.want, *got)
			}
		})
	}
}

func stringPtr(value string) *string {
	return &value
}
