//go:build linux

package plugin_manager

import (
	"fmt"
	goplugin "plugin"
)

func loadNativePluginDefinition(runtimePath string) (PluginDefinition, error) {
	pluginHandle, err := goplugin.Open(runtimePath)
	if err != nil {
		return PluginDefinition{}, fmt.Errorf("failed to open native plugin: %w", err)
	}

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
