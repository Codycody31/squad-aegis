package discord_admin_cam_logs

import "go.codycody31.dev/squad-aegis/internal/extension_manager"

// DiscordAdminCamLogsRegistrar implements the ExtensionRegistrar interface
type DiscordAdminCamLogsRegistrar struct{}

func (r DiscordAdminCamLogsRegistrar) Define() extension_manager.ExtensionDefinition {
	return Define()
}
