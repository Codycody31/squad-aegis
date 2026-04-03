//go:build !linux

package plugin_manager

import "fmt"

func loadNativePluginDefinition(runtimePath string) (PluginDefinition, error) {
	return PluginDefinition{}, fmt.Errorf("native plugins are only supported on Linux")
}
