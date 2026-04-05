//go:build linux

package plugin_manager

import (
	"fmt"
	goplugin "plugin"
)

const nativeConnectorEntrySymbol = "GetAegisConnector"

func loadNativeConnectorDefinition(runtimePath string) (ConnectorDefinition, error) {
	pluginHandle, err := goplugin.Open(runtimePath)
	if err != nil {
		return ConnectorDefinition{}, fmt.Errorf("failed to open native connector: %w", err)
	}

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
