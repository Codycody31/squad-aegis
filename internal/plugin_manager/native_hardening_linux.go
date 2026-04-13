//go:build linux

package plugin_manager

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	goplugin "github.com/hashicorp/go-plugin"
	"github.com/rs/zerolog/log"

	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

// applySubprocessHardening configures the SysProcAttr of a plugin/connector
// subprocess command to drop privileges before exec. It is a no-op when the
// operator has not configured a dedicated UID/GID.
//
// Note on PR_SET_NO_NEW_PRIVS: Go's exec.Cmd does not currently expose a
// SysProcAttr field to set this prctl. Operators who need the guarantee
// should run the whole Aegis process tree under a systemd unit with
// NoNewPrivileges=yes (or an equivalent container flag) — the prctl is
// inherited across exec into subprocess children. The config knob
// Plugins.SubprocessNoNewPrivs is therefore advisory and logged at startup.
func applySubprocessHardening(cmd *exec.Cmd) error {
	if cmd == nil {
		return nil
	}
	uid := 0
	gid := 0
	groupsCSV := ""
	if config.Config != nil {
		uid = config.Config.Plugins.SubprocessUID
		gid = config.Config.Plugins.SubprocessGID
		groupsCSV = config.Config.Plugins.SubprocessGroups
	}

	attr := cmd.SysProcAttr
	if attr == nil {
		attr = &syscall.SysProcAttr{}
	}
	// Start in a new process group so we can signal the whole tree on kill.
	attr.Setpgid = true

	if uid > 0 {
		effectiveGid := gid
		if effectiveGid <= 0 {
			effectiveGid = uid
		}
		groups, err := parseSupplementaryGroups(groupsCSV)
		if err != nil {
			return fmt.Errorf("parse subprocess supplementary groups: %w", err)
		}
		attr.Credential = &syscall.Credential{
			Uid:         uint32(uid),
			Gid:         uint32(effectiveGid),
			Groups:      groups,
			NoSetGroups: groups == nil,
		}
	}

	cmd.SysProcAttr = attr
	return nil
}

// logSubprocessHardeningPosture prints a single-line summary at startup so
// operators can see whether subprocess privilege drops are active.
func logSubprocessHardeningPosture() {
	if config.Config == nil {
		return
	}
	cfg := config.Config.Plugins
	if cfg.SubprocessUID > 0 {
		log.Info().
			Int("uid", cfg.SubprocessUID).
			Int("gid", cfg.SubprocessGID).
			Str("groups", cfg.SubprocessGroups).
			Msg("Native plugin subprocesses will exec with a dedicated UID/GID")
	} else {
		log.Warn().Msg("Native plugin subprocesses will inherit the Aegis process UID. Set Plugins.SubprocessUID for stronger isolation.")
	}
	if cfg.SubprocessNoNewPrivs {
		log.Info().Msg("Plugins.SubprocessNoNewPrivs is advisory: run the Aegis process itself under a systemd unit with NoNewPrivileges=yes (or equivalent) so the prctl is inherited into subprocesses.")
	}
}

// killProcessGroup sends SIGKILL to the entire process group of the
// go-plugin client subprocess, catching any forked children. Errors are
// ignored since the process may have already exited.
func killProcessGroup(client *goplugin.Client) {
	if client == nil {
		return
	}
	rc := client.ReattachConfig()
	if rc.Pid > 0 {
		// Kill the entire process group to catch forked children.
		_ = syscall.Kill(-rc.Pid, syscall.SIGKILL)
	}
}

// parseSupplementaryGroups parses a comma-separated list of numeric group
// IDs into a []uint32 suitable for syscall.Credential.Groups. Empty string
// returns a nil slice so NoSetGroups can be used to skip the setgroups(2)
// call entirely.
func parseSupplementaryGroups(csv string) ([]uint32, error) {
	trimmed := strings.TrimSpace(csv)
	if trimmed == "" {
		return nil, nil
	}
	parts := strings.Split(trimmed, ",")
	out := make([]uint32, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		gid, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid supplementary gid %q: %w", p, err)
		}
		out = append(out, uint32(gid))
	}
	return out, nil
}
