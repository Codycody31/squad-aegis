//go:build linux

package plugin_manager

import (
	"os/exec"
	"reflect"
	"testing"

	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

func TestParseSupplementaryGroups(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    []uint32
		wantErr bool
	}{
		{name: "empty", input: "", want: nil},
		{name: "whitespace", input: "  ", want: nil},
		{name: "single", input: "1001", want: []uint32{1001}},
		{name: "multiple", input: "1001,1002,1003", want: []uint32{1001, 1002, 1003}},
		{name: "spaces", input: "1001 , 1002 , 1003", want: []uint32{1001, 1002, 1003}},
		{name: "skip empty", input: "1001,,1002", want: []uint32{1001, 1002}},
		{name: "invalid", input: "1001,nope", wantErr: true},
		{name: "negative", input: "-1", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSupplementaryGroups(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseSupplementaryGroups(%q) err = nil, want error", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSupplementaryGroups(%q) err = %v", tc.input, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseSupplementaryGroups(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestApplySubprocessHardeningDefaultIsSetpgidOnly(t *testing.T) {
	prev := config.Config
	cfg := config.Struct{}
	config.Config = &cfg
	t.Cleanup(func() { config.Config = prev })

	cmd := exec.Command("/bin/true")
	if err := applySubprocessHardening(cmd); err != nil {
		t.Fatalf("applySubprocessHardening() error = %v", err)
	}
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil after hardening")
	}
	if !cmd.SysProcAttr.Setpgid {
		t.Fatal("Setpgid should always be true")
	}
	if cmd.SysProcAttr.Credential != nil {
		t.Fatalf("Credential = %+v, want nil when UID is unset", cmd.SysProcAttr.Credential)
	}
}

func TestApplySubprocessHardeningSetsCredentialWhenUIDConfigured(t *testing.T) {
	prev := config.Config
	cfg := config.Struct{}
	cfg.Plugins.SubprocessUID = 1001
	cfg.Plugins.SubprocessGID = 1002
	cfg.Plugins.SubprocessGroups = "1003,1004"
	config.Config = &cfg
	t.Cleanup(func() { config.Config = prev })

	cmd := exec.Command("/bin/true")
	if err := applySubprocessHardening(cmd); err != nil {
		t.Fatalf("applySubprocessHardening() error = %v", err)
	}
	cred := cmd.SysProcAttr.Credential
	if cred == nil {
		t.Fatal("Credential is nil, want populated")
	}
	if cred.Uid != 1001 {
		t.Fatalf("Uid = %d, want 1001", cred.Uid)
	}
	if cred.Gid != 1002 {
		t.Fatalf("Gid = %d, want 1002", cred.Gid)
	}
	if !reflect.DeepEqual(cred.Groups, []uint32{1003, 1004}) {
		t.Fatalf("Groups = %v, want [1003 1004]", cred.Groups)
	}
	if cred.NoSetGroups {
		t.Fatal("NoSetGroups should be false when supplementary groups are provided")
	}
}

func TestApplySubprocessHardeningGIDDefaultsToUID(t *testing.T) {
	prev := config.Config
	cfg := config.Struct{}
	cfg.Plugins.SubprocessUID = 2020
	// GID left at zero
	config.Config = &cfg
	t.Cleanup(func() { config.Config = prev })

	cmd := exec.Command("/bin/true")
	if err := applySubprocessHardening(cmd); err != nil {
		t.Fatalf("applySubprocessHardening() error = %v", err)
	}
	cred := cmd.SysProcAttr.Credential
	if cred == nil {
		t.Fatal("Credential is nil")
	}
	if cred.Gid != 2020 {
		t.Fatalf("Gid = %d, want 2020 (defaulted to UID)", cred.Gid)
	}
	if cred.NoSetGroups {
		t.Fatal("NoSetGroups must stay false so setgroups(2) clears inherited parent supplementary groups")
	}
	if cred.Groups == nil {
		t.Fatal("Groups must be a non-nil (possibly empty) slice so Go invokes setgroups(2)")
	}
	if len(cred.Groups) != 0 {
		t.Fatalf("Groups = %v, want empty slice when no supplementary groups configured", cred.Groups)
	}
}
