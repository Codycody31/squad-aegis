package discord_fob_hab_explosion_damage

import "go.codycody31.dev/squad-aegis/internal/extension_manager"

// DiscordFOBHabExplosionDamageRegistrar implements the ExtensionRegistrar interface
type DiscordFOBHabExplosionDamageRegistrar struct{}

func (r DiscordFOBHabExplosionDamageRegistrar) Define() extension_manager.ExtensionDefinition {
	return Define()
}
