//go:build !linux

package plugin_manager

import (
	"os/exec"

	goplugin "github.com/hashicorp/go-plugin"
)

// applySubprocessHardening is a no-op on non-linux platforms. Native
// plugins are Linux-only in production, so there is no portable way to
// drop privileges here.
func applySubprocessHardening(cmd *exec.Cmd) (func(), error) { return func() {}, nil }

// logSubprocessHardeningPosture is a no-op on non-linux platforms.
func logSubprocessHardeningPosture() {}

// killProcessGroup is a no-op on non-linux platforms.
func killProcessGroup(_ *goplugin.Client) {}
