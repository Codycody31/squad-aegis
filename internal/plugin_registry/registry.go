package plugin_registry

import (
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/connectors/discord"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/plugins/discord_admin_request"
)

// RegisterAllPlugins registers all available plugins with the plugin manager
func RegisterAllPlugins(pm *plugin_manager.PluginManager) error {
	log.Info().Msg("Registering plugins...")

	// Register Discord Admin Request plugin
	if err := pm.RegisterPlugin(discord_admin_request.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Discord Admin Request plugin")
		return err
	}

	log.Info().Msg("All plugins registered successfully")
	return nil
}

// RegisterAllConnectors registers all available connectors with the plugin manager
func RegisterAllConnectors(pm *plugin_manager.PluginManager) error {
	log.Info().Msg("Registering connectors...")

	// Register Discord connector
	if err := pm.RegisterConnector(discord.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Discord connector")
		return err
	}

	log.Info().Msg("All connectors registered successfully")
	return nil
}
