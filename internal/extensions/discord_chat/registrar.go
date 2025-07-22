package discord_chat

import "go.codycody31.dev/squad-aegis/internal/extension_manager"

// DiscordChatRegistrar implements the ExtensionRegistrar interface
type DiscordChatRegistrar struct{}

func (r DiscordChatRegistrar) Define() extension_manager.ExtensionDefinition {
	return Define()
}
