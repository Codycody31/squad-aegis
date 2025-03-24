package discord_teamkill

import "go.codycody31.dev/squad-aegis/internal/extension_manager"

// DiscordTeamkillRegistrar implements the ExtensionRegistrar interface
type DiscordTeamkillRegistrar struct{}

func (r DiscordTeamkillRegistrar) Define() extension_manager.ExtensionDefinition {
	return Define()
}
