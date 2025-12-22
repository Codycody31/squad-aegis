package discord_teamkill

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/connectors/discord"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// DiscordTeamkillPlugin logs all wounds and related information to Discord
type DiscordTeamkillPlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// Discord connector
	discordAPI discord.DiscordAPI

	// State management
	mu     sync.Mutex
	status plugin_manager.PluginStatus
	ctx    context.Context
	cancel context.CancelFunc
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "discord_teamkill",
		Name:                   "Discord Teamkill",
		Description:            "The Discord Teamkill plugin logs all wounds and related information to a Discord channel.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{"discord"},
		LongRunning:            false,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "channel_id",
					Description: "The ID of the channel to log teamkills to.",
					Required:    true,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "",
				},
				{
					Name:        "color",
					Description: "The color of the embeds.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     16761867, // Orange color
				},
				{
					Name:        "disable_cbl",
					Description: "Disable Community Ban List information.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     false,
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeLogPlayerWounded,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &DiscordTeamkillPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *DiscordTeamkillPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *DiscordTeamkillPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	p.apis = apis
	p.status = plugin_manager.PluginStatusStopped

	// Validate config
	definition := p.GetDefinition()
	if err := definition.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Fill defaults
	definition.ConfigSchema.FillDefaults(config)

	// Get Discord connector
	discordConnector, err := apis.ConnectorAPI.GetConnector("discord")
	if err != nil {
		return fmt.Errorf("failed to get Discord connector: %w", err)
	}

	// Type assertion
	var ok bool
	p.discordAPI, ok = discordConnector.(discord.DiscordAPI)
	if !ok {
		return fmt.Errorf("invalid Discord connector type")
	}

	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// Start begins plugin execution (for long-running plugins)
func (p *DiscordTeamkillPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	// Validate channel ID
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id is required but not configured")
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	return nil
}

// Stop gracefully stops the plugin
func (p *DiscordTeamkillPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusStopped {
		return nil // Already stopped
	}

	p.status = plugin_manager.PluginStatusStopping

	if p.cancel != nil {
		p.cancel()
	}

	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *DiscordTeamkillPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != "LOG_PLAYER_WOUNDED" {
		return nil // Not interested in this event
	}

	return p.handlePlayerWounded(event)
}

// GetStatus returns the current plugin status
func (p *DiscordTeamkillPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *DiscordTeamkillPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *DiscordTeamkillPlugin) UpdateConfig(config map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Validate new config
	definition := p.GetDefinition()
	if err := definition.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Fill defaults
	definition.ConfigSchema.FillDefaults(config)

	p.config = config

	return nil
}

// handlePlayerWounded processes player wounded events
func (p *DiscordTeamkillPlugin) handlePlayerWounded(rawEvent *plugin_manager.PluginEvent) error {
	event, ok := rawEvent.Data.(*event_manager.LogPlayerWoundedData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	// Check if we have attacker information
	if event.AttackerPlayerController == "" || !event.Teamkill {
		return nil // No attacker information
	}

	// Send Discord embed in a goroutine to avoid blocking
	go func() {
		if err := p.sendTeamkillEmbed(event); err != nil {
			p.apis.LogAPI.Error("Failed to send Discord embed for Teamkill", err, map[string]interface{}{
				"victim_name":                event.VictimName,
				"attacker_player_controller": event.AttackerPlayerController,
				"weapon":                     event.Weapon,
			})
		}
	}()

	return nil
}

// sendTeamkillEmbed sends the Teamkill as a Discord embed
func (p *DiscordTeamkillPlugin) sendTeamkillEmbed(event *event_manager.LogPlayerWoundedData) error {
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id not configured")
	}

	// Extract attacker name from player controller (fallback if no separate name field)
	attackerName := event.Attacker.PlayerSuffix
	attackerSteamID := event.Attacker.SteamID
	attackerEOSID := event.Attacker.EOSID
	victimName := event.Victim.PlayerSuffix
	victimSteamID := event.Victim.SteamID
	victimEOSID := event.Victim.EOSID

	// Default values for missing data
	if attackerSteamID == "" {
		attackerSteamID = "Unknown"
	}
	if attackerEOSID == "" {
		attackerEOSID = "Unknown"
	}
	if victimName == "" {
		victimName = "Unknown"
	}
	if victimSteamID == "" {
		victimSteamID = "Unknown"
	}
	if victimEOSID == "" {
		victimEOSID = "Unknown"
	}

	fields := []*discord.DiscordEmbedField{
		{
			Name:   "Attacker's Name",
			Value:  attackerName,
			Inline: true,
		},
		{
			Name:   "Attacker's SteamID",
			Value:  fmt.Sprintf("[%s](https://steamcommunity.com/profiles/%s)", attackerSteamID, attackerSteamID),
			Inline: true,
		},
		{
			Name:   "Attacker's EosID",
			Value:  attackerEOSID,
			Inline: true,
		},
		{
			Name:  "Weapon",
			Value: event.Weapon,
		},
		{
			Name:   "Victim's Name",
			Value:  victimName,
			Inline: true,
		},
		{
			Name:   "Victim's SteamID",
			Value:  victimSteamID,
			Inline: true,
		},
		{
			Name:   "Victim's EosID",
			Value:  victimEOSID,
			Inline: true,
		},
	}

	// Add Community Ban List link if not disabled
	if !p.getBoolConfig("disable_cbl") && attackerSteamID != "Unknown" {
		fields = append(fields, &discord.DiscordEmbedField{
			Name:  "Community Ban List",
			Value: fmt.Sprintf("[Attacker's Bans](https://communitybanlist.com/search/%s)", attackerSteamID),
		})
	}

	title := fmt.Sprintf("Teamkill: %s", attackerName)

	embed := &discord.DiscordEmbed{
		Title:     title,
		Color:     p.getIntConfig("color"),
		Fields:    fields,
		Timestamp: func() *time.Time { t := time.Now(); return &t }(),
	}

	if _, err := p.discordAPI.SendEmbed(channelID, embed); err != nil {
		return fmt.Errorf("failed to send Discord embed: %w", err)
	}

	return nil
}

// Helper methods for config access

func (p *DiscordTeamkillPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *DiscordTeamkillPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *DiscordTeamkillPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}
