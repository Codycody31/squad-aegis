package discord_killfeed

import "go.codycody31.dev/squad-aegis/internal/extension_manager"

// DiscordKillfeedRegistrar implements the ExtensionRegistrar interface
type DiscordKillfeedRegistrar struct{}

func (r DiscordKillfeedRegistrar) Define() extension_manager.ExtensionDefinition {
	return Define()
}
