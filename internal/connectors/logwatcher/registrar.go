package logwatcher

import "go.codycody31.dev/squad-aegis/internal/connector_manager"

// LogWatcherRegistrar implements the ConnectorRegistrar interface
type LogWatcherRegistrar struct{}

// Define returns the connector definition
func (r LogWatcherRegistrar) Define() connector_manager.ConnectorDefinition {
	return Define()
}

// Registrar is the singleton instance for registration
var Registrar = &LogWatcherRegistrar{}
