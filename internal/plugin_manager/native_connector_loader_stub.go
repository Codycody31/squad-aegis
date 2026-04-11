//go:build !linux

package plugin_manager

import "fmt"

func loadNativeConnectorDefinition(runtimePath string) (ConnectorDefinition, error) {
	return ConnectorDefinition{}, fmt.Errorf("native connectors are only supported on Linux")
}

func loadNativeConnectorDefinitionFromFD(fd uintptr) (ConnectorDefinition, error) {
	return ConnectorDefinition{}, fmt.Errorf("native connectors are only supported on Linux")
}
