//go:build unix

package plugin_manager

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeRuntimeLibraryPlatform writes a native plugin/connector runtime
// binary to a temp file in the destination directory and atomically renames
// it into place. This is race-free against a concurrent attacker who might
// pre-create the destination path: O_EXCL protects the temp file at
// creation, and rename(2) atomically replaces the final path without a
// Remove/Create window. The final file mode includes the executable bit
// so the host can fork/exec it via hashicorp/go-plugin.
func writeRuntimeLibraryPlatform(runtimePath string, libraryBytes []byte) error {
	dir := filepath.Dir(runtimePath)
	base := filepath.Base(runtimePath)

	// CreateTemp uses O_EXCL internally, so the fd is safe from symlink
	// attacks at creation time. We write directly to this fd rather than
	// closing and reopening (which would introduce a TOCTOU window where
	// an attacker could replace the temp file with a symlink).
	tempFile, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp runtime binary in %s: %w", dir, err)
	}
	tempPath := tempFile.Name()
	// Best-effort cleanup if anything below this point fails.
	cleanup := func() { _ = os.Remove(tempPath) }
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	if _, writeErr := tempFile.Write(libraryBytes); writeErr != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to write temp runtime binary %s: %w", tempPath, writeErr)
	}
	// 0o750 grants owner rwx + group rx, so the host user can fork/exec.
	// World has no access. Adjust via systemd unit if a dedicated non-host
	// user needs to read the binary.
	if err := tempFile.Chmod(0o750); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to chmod temp runtime binary %s: %w", tempPath, err)
	}
	if syncErr := tempFile.Sync(); syncErr != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to sync temp runtime binary %s: %w", tempPath, syncErr)
	}
	if closeErr := tempFile.Close(); closeErr != nil {
		return fmt.Errorf("failed to finalize temp runtime binary %s: %w", tempPath, closeErr)
	}

	// rename(2) atomically replaces any existing file at runtimePath. No
	// Remove → Create window means a concurrent attacker cannot insert a
	// decoy at the destination path.
	if err := os.Rename(tempPath, runtimePath); err != nil {
		return fmt.Errorf("failed to install runtime library %s: %w", runtimePath, err)
	}
	cleanup = nil
	return nil
}
