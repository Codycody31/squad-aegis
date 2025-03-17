package discord_squad_created

import "go.codycody31.dev/squad-aegis/internal/extension_manager"

// DiscordSquadCreatedRegistrar implements the ExtensionRegistrar interface
type DiscordSquadCreatedRegistrar struct{}

func (r DiscordSquadCreatedRegistrar) Define() extension_manager.ExtensionDefinition {
	return Define()
}
