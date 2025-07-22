package discord_admin_broadcast

import "go.codycody31.dev/squad-aegis/internal/extension_manager"

// DiscordAdminBroadcastRegistrar implements the ExtensionRegistrar interface
type DiscordAdminBroadcastRegistrar struct{}

func (r DiscordAdminBroadcastRegistrar) Define() extension_manager.ExtensionDefinition {
	return Define()
}
