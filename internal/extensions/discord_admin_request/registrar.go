package discord_admin_request

import "go.codycody31.dev/squad-aegis/internal/extension_manager"

// DiscordAdminRequestRegistrar implements the ExtensionRegistrar interface
type DiscordAdminRequestRegistrar struct{}

func (r DiscordAdminRequestRegistrar) Define() extension_manager.ExtensionDefinition {
	return Define()
}
