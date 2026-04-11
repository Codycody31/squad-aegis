//go:build linux

package plugin_manager

import (
	"fmt"
	goplugin "plugin"
)

func loadNativeConnectorDefinition(runtimePath string) (ConnectorDefinition, error) {
	pluginHandle, err := goplugin.Open(runtimePath)
	if err != nil {
		return ConnectorDefinition{}, fmt.Errorf("failed to open native connector: %w", err)
	}
	return resolveConnectorEntry(pluginHandle)
}

// loadNativeConnectorDefinitionFromFD opens the connector via /proc/self/fd/N
// so the inode bound to fd is the one dlopened. Caller must keep fd open.
func loadNativeConnectorDefinitionFromFD(fd uintptr) (ConnectorDefinition, error) {
	procPath := fmt.Sprintf("/proc/self/fd/%d", fd)
	pluginHandle, err := goplugin.Open(procPath)
	if err != nil {
		return ConnectorDefinition{}, fmt.Errorf("failed to open native connector via fd: %w", err)
	}
	return resolveConnectorEntry(pluginHandle)
}

func resolveConnectorEntry(pluginHandle *goplugin.Plugin) (ConnectorDefinition, error) {
	symbol, err := pluginHandle.Lookup(nativeConnectorEntrySymbol)
	if err != nil {
		return ConnectorDefinition{}, fmt.Errorf("failed to resolve %s: %w", nativeConnectorEntrySymbol, err)
	}

	switch getConnector := symbol.(type) {
	case func() ConnectorDefinition:
		return getConnector(), nil
	case *func() ConnectorDefinition:
		return (*getConnector)(), nil
	default:
		return ConnectorDefinition{}, fmt.Errorf("%s has an incompatible signature", nativeConnectorEntrySymbol)
	}
}
