package heartbeat

import "go.codycody31.dev/squad-aegis/internal/connector_manager"

// HeartbeatRegistrar implements the ConnectorRegistrar interface
type HeartbeatRegistrar struct{}

// Define returns the connector definition
func (r HeartbeatRegistrar) Define() connector_manager.ConnectorDefinition {
	return Define()
}

// Registrar is the singleton instance for registration
var Registrar = &HeartbeatRegistrar{}
