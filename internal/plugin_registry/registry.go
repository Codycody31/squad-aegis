package plugin_registry

import (
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/connectors/discord"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/plugins/auto_kick_unassigned"
	"go.codycody31.dev/squad-aegis/internal/plugins/auto_tk_warn"
	"go.codycody31.dev/squad-aegis/internal/plugins/auto_warn_sl_wrong_kit"
	"go.codycody31.dev/squad-aegis/internal/plugins/cbl"
	"go.codycody31.dev/squad-aegis/internal/plugins/chat_commands"
	"go.codycody31.dev/squad-aegis/internal/plugins/command_scheduler"
	"go.codycody31.dev/squad-aegis/internal/plugins/discord_admin_broadcast"
	"go.codycody31.dev/squad-aegis/internal/plugins/discord_admin_cam_logs"
	"go.codycody31.dev/squad-aegis/internal/plugins/discord_admin_request"
	"go.codycody31.dev/squad-aegis/internal/plugins/discord_chat"
	"go.codycody31.dev/squad-aegis/internal/plugins/discord_fob_hab_explosion_damage"
	"go.codycody31.dev/squad-aegis/internal/plugins/discord_kill_feed"
	"go.codycody31.dev/squad-aegis/internal/plugins/discord_round_ended"
	"go.codycody31.dev/squad-aegis/internal/plugins/discord_round_winner"
	"go.codycody31.dev/squad-aegis/internal/plugins/discord_squad_created"
	"go.codycody31.dev/squad-aegis/internal/plugins/fog_of_war"
	"go.codycody31.dev/squad-aegis/internal/plugins/intervalled_broadcasts"
	"go.codycody31.dev/squad-aegis/internal/plugins/kill_broadcast"
	"go.codycody31.dev/squad-aegis/internal/plugins/rule_lookup"
	"go.codycody31.dev/squad-aegis/internal/plugins/seeding_mode"
	"go.codycody31.dev/squad-aegis/internal/plugins/server_seeder_whitelist"
	"go.codycody31.dev/squad-aegis/internal/plugins/squad_leader_whitelist"
	"go.codycody31.dev/squad-aegis/internal/plugins/switch_teams"
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

	// Register Auto Kick SL Wrong Kit plugin
	if err := pm.RegisterPlugin(auto_warn_sl_wrong_kit.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Auto Kick SL Wrong Kit plugin")
		return err
	}

	// Register Command Scheduler plugin
	if err := pm.RegisterPlugin(command_scheduler.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Command Scheduler plugin")
		return err
	}

	// Register Auto TK Warn plugin
	if err := pm.RegisterPlugin(auto_tk_warn.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Auto TK Warn plugin")
		return err
	}

	// Register CBL plugin
	if err := pm.RegisterPlugin(cbl.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register CBL plugin")
		return err
	}

	// Register Chat Commands plugin
	if err := pm.RegisterPlugin(chat_commands.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Chat Commands plugin")
		return err
	}

	// Register Discord Admin Broadcast plugin
	if err := pm.RegisterPlugin(discord_admin_broadcast.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Discord Admin Broadcast plugin")
		return err
	}

	// Register Discord Admin Camera Logs plugin
	if err := pm.RegisterPlugin(discord_admin_cam_logs.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Discord Admin Camera Logs plugin")
		return err
	}

	// Register Discord Chat plugin
	if err := pm.RegisterPlugin(discord_chat.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Discord Chat plugin")
		return err
	}

	// Register Discord FOB/HAB Explosion Damage plugin
	if err := pm.RegisterPlugin(discord_fob_hab_explosion_damage.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Discord FOB/HAB Explosion Damage plugin")
		return err
	}

	// Register Discord Kill Feed plugin
	if err := pm.RegisterPlugin(discord_kill_feed.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Discord Kill Feed plugin")
		return err
	}

	// Register Discord Round Winner plugin
	if err := pm.RegisterPlugin(discord_round_winner.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Discord Round Winner plugin")
		return err
	}

	// Register Discord Round Ended plugin
	if err := pm.RegisterPlugin(discord_round_ended.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Discord Round Ended plugin")
		return err
	}

	// Register Discord Squad Created plugin
	if err := pm.RegisterPlugin(discord_squad_created.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Discord Squad Created plugin")
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

	// Register Switch Teams plugin
	if err := pm.RegisterPlugin(switch_teams.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Switch Teams plugin")
		return err
	}

	// Register Server Seeder Whitelist plugin
	if err := pm.RegisterPlugin(server_seeder_whitelist.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Server Seeder Whitelist plugin")
		return err
	}

	// Register Squad Leader Whitelist plugin
	if err := pm.RegisterPlugin(squad_leader_whitelist.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Squad Leader Whitelist plugin")
		return err
	}

	// Register Rule Lookup plugin
	if err := pm.RegisterPlugin(rule_lookup.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Rule Lookup plugin")
		return err
	}

	// Register Kill Broadcast plugin
	if err := pm.RegisterPlugin(kill_broadcast.Define()); err != nil {
		log.Error().Err(err).Msg("Failed to register Kill Broadcast plugin")
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
