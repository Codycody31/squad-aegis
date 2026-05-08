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

func intPtr(value int) *int {
	return &value
}

func TestValidateIPAddress(t *testing.T) {
	t.Parallel()

	valid := []string{
		"127.0.0.1",
		"192.168.1.1",
		"::1",
		"2001:db8::1",
	}
	for _, ip := range valid {
		if err := validateIPAddress(ip); err != nil {
			t.Errorf("expected %q to be valid, got: %v", ip, err)
		}
	}

	invalid := []string{
		"node.example.com",
		"localhost",
		"999.999.999.999",
		"not-an-ip",
		"1.2.3",
	}
	for _, ip := range invalid {
		if err := validateIPAddress(ip); err == nil {
			t.Errorf("expected %q to be invalid", ip)
		}
	}

	// Empty string is allowed (Required is a separate check).
	if err := validateIPAddress(""); err != nil {
		t.Errorf("expected empty string to pass (Required handles emptiness), got: %v", err)
	}
}

func TestValidateOptionalIPAddress(t *testing.T) {
	t.Parallel()

	if err := validateOptionalIPAddress((*string)(nil)); err != nil {
		t.Errorf("nil pointer should be valid, got: %v", err)
	}
	good := "10.0.0.1"
	if err := validateOptionalIPAddress(&good); err != nil {
		t.Errorf("expected valid IP to pass, got: %v", err)
	}
	bad := "panel.example.com"
	if err := validateOptionalIPAddress(&bad); err == nil {
		t.Error("expected hostname to fail validation")
	}
}

func TestValidateLogPort(t *testing.T) {
	t.Parallel()

	if err := validateLogPort((*int)(nil)); err != nil {
		t.Errorf("nil pointer should be valid, got: %v", err)
	}
	for _, p := range []int{1, 22, 2022, 65535} {
		if err := validateLogPort(intPtr(p)); err != nil {
			t.Errorf("port %d should be valid, got: %v", p, err)
		}
	}
	for _, p := range []int{0, -1, 65536, 99999} {
		if err := validateLogPort(intPtr(p)); err == nil {
			t.Errorf("port %d should be invalid", p)
		}
	}
}
