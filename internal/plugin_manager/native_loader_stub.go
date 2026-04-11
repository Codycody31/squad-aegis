//go:build !linux

package plugin_manager

import (
	"fmt"
	"os"
)

func loadNativePluginDefinition(runtimePath string) (PluginDefinition, error) {
	return PluginDefinition{}, fmt.Errorf("native plugins are only supported on Linux")
}

func loadNativePluginDefinitionFromFD(fd uintptr) (PluginDefinition, error) {
	return PluginDefinition{}, fmt.Errorf("native plugins are only supported on Linux")
}

func openNoFollow(runtimePath string) (*os.File, error) {
	return nil, fmt.Errorf("native plugins are only supported on Linux")
}
