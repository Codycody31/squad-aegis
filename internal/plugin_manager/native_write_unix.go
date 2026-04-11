//go:build unix

package plugin_manager

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// writeRuntimeLibraryPlatform writes a native plugin/connector runtime
// binary to a temp file in the destination directory and atomically renames
// it into place. This is race-free against a concurrent attacker who might
// pre-create the destination path: O_EXCL + O_NOFOLLOW protects the temp
// file, and rename(2) atomically replaces the final path without a
// Remove/Create window. The final file mode includes the executable bit
// so the host can fork/exec it via hashicorp/go-plugin.
func writeRuntimeLibraryPlatform(runtimePath string, libraryBytes []byte) error {
	dir := filepath.Dir(runtimePath)
	base := filepath.Base(runtimePath)

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

	// Re-open with O_NOFOLLOW so we never write through a symlink that may
	// have replaced the temp file between CreateTemp and now.
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close initial temp runtime binary handle %s: %w", tempPath, err)
	}
	flags := os.O_WRONLY | os.O_TRUNC | syscall.O_NOFOLLOW
	file, err := os.OpenFile(tempPath, flags, 0o750)
	if err != nil {
		return fmt.Errorf("failed to reopen temp runtime binary %s: %w", tempPath, err)
	}

	if _, writeErr := file.Write(libraryBytes); writeErr != nil {
		_ = file.Close()
		return fmt.Errorf("failed to write temp runtime binary %s: %w", tempPath, writeErr)
	}
	if syncErr := file.Sync(); syncErr != nil {
		_ = file.Close()
		return fmt.Errorf("failed to sync temp runtime binary %s: %w", tempPath, syncErr)
	}
	if closeErr := file.Close(); closeErr != nil {
		return fmt.Errorf("failed to finalize temp runtime binary %s: %w", tempPath, closeErr)
	}
	// 0o750 grants owner rwx + group rx, so the host user can fork/exec.
	// World has no access. Adjust via systemd unit if a dedicated non-host
	// user needs to read the binary.
	if err := os.Chmod(tempPath, 0o750); err != nil {
		return fmt.Errorf("failed to chmod temp runtime binary %s: %w", tempPath, err)
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
