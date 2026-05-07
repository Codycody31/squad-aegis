//go:build !unix

package plugin_manager

import (
	"fmt"
	"os"
)

// openNoFollow is the non-unix fallback. O_NOFOLLOW is not available on all
// platforms; best-effort open without symlink hardening. Native plugins are
// only supported on Linux in practice.
func openNoFollow(runtimePath string) (*os.File, error) {
	file, err := os.OpenFile(runtimePath, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open native runtime file %s: %w", runtimePath, err)
	}
	return file, nil
}
