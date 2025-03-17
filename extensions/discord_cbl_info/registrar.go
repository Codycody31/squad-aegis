package discord_cbl_info

import "go.codycody31.dev/squad-aegis/internal/extension_manager"

// DiscordCBLInfoRegistrar implements the ExtensionRegistrar interface
type DiscordCBLInfoRegistrar struct{}

func (r DiscordCBLInfoRegistrar) Define() extension_manager.ExtensionDefinition {
	return Define()
}
