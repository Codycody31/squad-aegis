//go:build !linux

package plugin_manager

import "os/exec"

// applySubprocessHardening is a no-op on non-linux platforms. Native
// plugins are Linux-only in production, so there is no portable way to
// drop privileges here.
func applySubprocessHardening(cmd *exec.Cmd) error { return nil }

// logSubprocessHardeningPosture is a no-op on non-linux platforms.
func logSubprocessHardeningPosture() {}
