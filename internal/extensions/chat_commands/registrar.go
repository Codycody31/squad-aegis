package chat_commands

import "go.codycody31.dev/squad-aegis/internal/extension_manager"

// ChatCommandsRegistrar implements the ExtensionRegistrar interface
type ChatCommandsRegistrar struct{}

func (r ChatCommandsRegistrar) Define() extension_manager.ExtensionDefinition {
	return Define()
}
