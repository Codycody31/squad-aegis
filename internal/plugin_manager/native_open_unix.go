//go:build unix

package plugin_manager

import (
	"fmt"
	"os"
	"syscall"
)

// openNoFollow opens a file with O_RDONLY|O_NOFOLLOW so a symlink on the
// final path component cannot redirect us to a different inode. Used for
// checksum verification of native plugin/connector runtime binaries.
func openNoFollow(runtimePath string) (*os.File, error) {
	flags := os.O_RDONLY | syscall.O_NOFOLLOW
	file, err := os.OpenFile(runtimePath, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open native runtime file %s: %w", runtimePath, err)
	}
	return file, nil
}
