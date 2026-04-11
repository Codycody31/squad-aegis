//go:build !unix

package plugin_manager

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeRuntimeLibraryPlatform writes via temp file + rename for atomicity.
// Non-unix targets cannot use O_NOFOLLOW, so this is best-effort and any
// production deployment is expected to be on Linux.
func writeRuntimeLibraryPlatform(runtimePath string, libraryBytes []byte) error {
	dir := filepath.Dir(runtimePath)
	base := filepath.Base(runtimePath)

	tempFile, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp runtime library in %s: %w", dir, err)
	}
	tempPath := tempFile.Name()
	cleanup := func() { _ = os.Remove(tempPath) }
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	if _, writeErr := tempFile.Write(libraryBytes); writeErr != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to write temp runtime library %s: %w", tempPath, writeErr)
	}
	if syncErr := tempFile.Sync(); syncErr != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to sync temp runtime library %s: %w", tempPath, syncErr)
	}
	if closeErr := tempFile.Close(); closeErr != nil {
		return fmt.Errorf("failed to finalize temp runtime library %s: %w", tempPath, closeErr)
	}
	if err := os.Chmod(tempPath, 0o640); err != nil {
		return fmt.Errorf("failed to chmod temp runtime library %s: %w", tempPath, err)
	}

	if err := os.Rename(tempPath, runtimePath); err != nil {
		return fmt.Errorf("failed to install runtime library %s: %w", runtimePath, err)
	}
	cleanup = nil
	return nil
}
