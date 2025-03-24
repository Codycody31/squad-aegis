package auto_kick_unassigned

import "go.codycody31.dev/squad-aegis/internal/extension_manager"

// AutoKickUnassignedRegistrar implements the ExtensionRegistrar interface
type AutoKickUnassignedRegistrar struct{}

func (r AutoKickUnassignedRegistrar) Define() extension_manager.ExtensionDefinition {
	return Define()
}
