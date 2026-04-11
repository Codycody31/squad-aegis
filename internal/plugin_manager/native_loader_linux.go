//go:build linux

package plugin_manager

import (
	"fmt"
	"os"
	goplugin "plugin"
	"syscall"
)

// loadNativePluginDefinition is the legacy entry point used by tests that
// inject a path-based loader. Production callers should prefer
// loadVerifiedNativePlugin which closes the TOCTOU between hash and dlopen.
func loadNativePluginDefinition(runtimePath string) (PluginDefinition, error) {
	pluginHandle, err := goplugin.Open(runtimePath)
	if err != nil {
		return PluginDefinition{}, fmt.Errorf("failed to open native plugin: %w", err)
	}
	return resolvePluginEntry(pluginHandle)
}

// loadNativePluginDefinitionFromFD opens the plugin via /proc/self/fd/N so
// that the file inode bound to fd is the one dlopened, not whatever happens
// to be at runtimePath at the moment of dlopen. The caller must keep fd
// open across this call.
func loadNativePluginDefinitionFromFD(fd uintptr) (PluginDefinition, error) {
	procPath := fmt.Sprintf("/proc/self/fd/%d", fd)
	pluginHandle, err := goplugin.Open(procPath)
	if err != nil {
		return PluginDefinition{}, fmt.Errorf("failed to open native plugin via fd: %w", err)
	}
	return resolvePluginEntry(pluginHandle)
}

func resolvePluginEntry(pluginHandle *goplugin.Plugin) (PluginDefinition, error) {
	symbol, err := pluginHandle.Lookup(nativePluginEntrySymbol)
	if err != nil {
		return PluginDefinition{}, fmt.Errorf("failed to resolve %s: %w", nativePluginEntrySymbol, err)
	}

	switch getPlugin := symbol.(type) {
	case func() PluginDefinition:
		return getPlugin(), nil
	case *func() PluginDefinition:
		return (*getPlugin)(), nil
	default:
		return PluginDefinition{}, fmt.Errorf("%s has an incompatible signature", nativePluginEntrySymbol)
	}
}

// openNoFollow opens the file with O_NOFOLLOW so a symlink anywhere along
// the final path component cannot redirect us to a different inode. The
// caller is responsible for closing the returned file.
func openNoFollow(runtimePath string) (*os.File, error) {
	flags := os.O_RDONLY | syscall.O_NOFOLLOW
	file, err := os.OpenFile(runtimePath, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open native plugin file %s: %w", runtimePath, err)
	}
	return file, nil
}
