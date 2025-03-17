package discord

import (
	"go.codycody31.dev/squad-aegis/internal/connector_manager"
)

// DiscordRegistrar implements the ConnectorRegistrar interface
type DiscordRegistrar struct{}

// Define returns the connector definition
func (r DiscordRegistrar) Define() connector_manager.ConnectorDefinition {
	return Define()
}

// Registrar is the singleton instance for registration
var Registrar = &DiscordRegistrar{}
