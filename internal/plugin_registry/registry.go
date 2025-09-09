package plugin_registry

import (
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/connectors/discord"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/plugins/auto_kick_unassigned"
	"go.codycody31.dev/squad-aegis/internal/plugins/auto_tk_warn"
	"go.codycody31.dev/squad-aegis/internal/plugins/discord_admin_request"
	"go.codycody31.dev/squad-aegis/internal/plugins/fog_of_war"
	"go.codycody31.dev/squad-aegis/internal/plugins/intervalled_broadcasts"
	"go.codycody31.dev/squad-aegis/internal/plugins/seeding_mode"
	"go.codycody31.dev/squad-aegis/internal/plugins/team_randomizer"
)

// RegisterAllPlugins registers all available plugins with the plugin manager
func RegisterAllPlugins(pm *plugin_manager.PluginManager) error {
	log.Info().Msg("Registering plugins...")

	// Register Discord Admin Request plugin
	if err := pm.RegisterPlugin(discord_admin_request.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Discord Admin Request plugin")
		return err
	}

	// Register Auto Kick Unassigned plugin
	if err := pm.RegisterPlugin(auto_kick_unassigned.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Auto Kick Unassigned plugin")
		return err
	}

	// Register Auto TK Warn plugin
	if err := pm.RegisterPlugin(auto_tk_warn.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Auto TK Warn plugin")
		return err
	}

	// Register Team Randomizer plugin
	if err := pm.RegisterPlugin(team_randomizer.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Team Randomizer plugin")
		return err
	}

	// Register Seeding Mode plugin
	if err := pm.RegisterPlugin(seeding_mode.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Seeding Mode plugin")
		return err
	}

	// Register Intervalled Broadcasts plugin
	if err := pm.RegisterPlugin(intervalled_broadcasts.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Intervalled Broadcasts plugin")
		return err
	}

	// Register Fog of War plugin
	if err := pm.RegisterPlugin(fog_of_war.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Fog of War plugin")
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
