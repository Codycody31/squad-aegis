//go:build !unix

package plugin_manager

import (
	"fmt"
	"os"
)

func writeRuntimeLibraryPlatform(runtimePath string, libraryBytes []byte) error {
	file, err := os.OpenFile(runtimePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
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
