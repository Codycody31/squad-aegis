//go:build linux

package plugin_manager

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"

	goplugin "github.com/hashicorp/go-plugin"
	"github.com/rs/zerolog/log"

	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

// subprocessUIDInUseMu guards subprocessUIDInUse. The map keys are
// effective UIDs of currently-live plugin/connector subprocesses; values are
// reference counts (always 1 today, but the type leaves room for the future
// AllowMultipleInstances case if it is ever lifted to share a UID slot).
var (
	subprocessUIDInUseMu sync.Mutex
	subprocessUIDInUse   = map[uint32]int{}
)

// claimSubprocessUID reserves an effective UID slot for a new subprocess.
// Returns an error if the UID is already claimed by another live subprocess.
// The returned release function must be invoked exactly once when the
// subprocess exits to free the slot.
func claimSubprocessUID(uid uint32) (func(), error) {
	subprocessUIDInUseMu.Lock()
	defer subprocessUIDInUseMu.Unlock()
	if _, exists := subprocessUIDInUse[uid]; exists {
		return nil, fmt.Errorf("subprocess uid %d is already in use by another live native plugin/connector; allocate a distinct SubprocessUID per plugin to keep their net/rpc sockets isolated", uid)
	}
	subprocessUIDInUse[uid] = 1
	released := false
	return func() {
		subprocessUIDInUseMu.Lock()
		defer subprocessUIDInUseMu.Unlock()
		if released {
			return
		}
		released = true
		delete(subprocessUIDInUse, uid)
	}, nil
}

// applySubprocessHardening configures the SysProcAttr of a plugin/connector
// subprocess command to drop privileges before exec. It is a no-op when the
// operator has not configured a dedicated UID/GID.
//
// Note on PR_SET_NO_NEW_PRIVS: Go's exec.Cmd does not currently expose a
// SysProcAttr field to set this prctl. Operators who need the guarantee
// should run the whole Aegis process tree under a systemd unit with
// NoNewPrivileges=yes (or an equivalent container flag) — the prctl is
// inherited across exec into subprocess children.
//
// IPC authentication caveat: the host<->subprocess channel uses
// hashicorp/go-plugin over net/rpc (see pkg/pluginrpc, pkg/connectorrpc),
// which has no per-connection auth. AutoMTLS requires gRPC and is not
// available here. To prevent one compromised plugin from connecting to
// another's RPC socket, this function refuses to spawn two live subprocesses
// under the same effective SubprocessUID. Operators isolating multiple
// plugins must allocate distinct UIDs.
func applySubprocessHardening(cmd *exec.Cmd) (func(), error) {
	noopRelease := func() {}
	if cmd == nil {
		return noopRelease, nil
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
	// Kill the subprocess if the host process dies to prevent orphans.
	attr.Pdeathsig = syscall.SIGKILL

	release := noopRelease
	if uid > 0 {
		effectiveGid := gid
		if effectiveGid <= 0 {
			effectiveGid = uid
		}
		groups, err := parseSupplementaryGroups(groupsCSV)
		if err != nil {
			return noopRelease, fmt.Errorf("parse subprocess supplementary groups: %w", err)
		}
		if groups == nil {
			groups = []uint32{}
		}
		uidRelease, err := claimSubprocessUID(uint32(uid))
		if err != nil {
			return noopRelease, err
		}
		release = uidRelease
		attr.Credential = &syscall.Credential{
			Uid:    uint32(uid),
			Gid:    uint32(effectiveGid),
			Groups: groups,
		}
	}

	cmd.SysProcAttr = attr
	return release, nil
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
// returns a nil slice; callers are expected to substitute an empty
// (non-nil) slice so setgroups(2) is still invoked — otherwise the
// subprocess would inherit the parent's supplementary groups.
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
