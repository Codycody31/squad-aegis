//go:build unix

package plugin_manager

import (
	"fmt"
	"os"
	"syscall"
)

func writeRuntimeLibraryPlatform(runtimePath string, libraryBytes []byte) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL | syscall.O_NOFOLLOW
	file, err := os.OpenFile(runtimePath, flags, 0o640)
	if err != nil {
		return fmt.Errorf("failed to create runtime library %s: %w", runtimePath, err)
	}

	if _, writeErr := file.Write(libraryBytes); writeErr != nil {
		_ = file.Close()
		return fmt.Errorf("failed to write runtime library %s: %w", runtimePath, writeErr)
	}

	if closeErr := file.Close(); closeErr != nil {
		return fmt.Errorf("failed to finalize runtime library %s: %w", runtimePath, closeErr)
	}

	return nil
}
