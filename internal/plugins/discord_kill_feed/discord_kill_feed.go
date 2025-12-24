package discord_kill_feed

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

// DiscordKillFeedPlugin logs all wounds and related information to Discord
type DiscordKillFeedPlugin struct {
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
		ID:                     "discord_kill_feed",
		Name:                   "Discord Kill Feed",
		Description:            "The Discord Kill Feed plugin logs all wounds and related information to a Discord channel for admins to review.",
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
			return &DiscordKillFeedPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *DiscordKillFeedPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

func (p *DiscordKillFeedPlugin) GetCommands() []plugin_manager.PluginCommand {
	return []plugin_manager.PluginCommand{}
}

func (p *DiscordKillFeedPlugin) ExecuteCommand(commandID string, params map[string]interface{}) (*plugin_manager.CommandResult, error) {
	return nil, fmt.Errorf("no commands available")
}

func (p *DiscordKillFeedPlugin) GetCommandExecutionStatus(executionID string) (*plugin_manager.CommandExecutionStatus, error) {
	return nil, fmt.Errorf("no commands available")
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *DiscordKillFeedPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
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
func (p *DiscordKillFeedPlugin) Start(ctx context.Context) error {
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
func (p *DiscordKillFeedPlugin) Stop() error {
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
func (p *DiscordKillFeedPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != "LOG_PLAYER_WOUNDED" {
		return nil // Not interested in this event
	}

	return p.handlePlayerWounded(event)
}

// GetStatus returns the current plugin status
func (p *DiscordKillFeedPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *DiscordKillFeedPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *DiscordKillFeedPlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Discord Kill Feed plugin configuration updated", map[string]interface{}{
		"channel_id":  config["channel_id"],
		"color":       config["color"],
		"disable_cbl": config["disable_cbl"],
	})

	return nil
}

// handlePlayerWounded processes player wounded events
func (p *DiscordKillFeedPlugin) handlePlayerWounded(rawEvent *plugin_manager.PluginEvent) error {
	event, ok := rawEvent.Data.(*event_manager.LogPlayerWoundedData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	// Check if we have attacker information
	if event.AttackerPlayerController == "" {
		return nil // No attacker information
	}

	// Send Discord embed in a goroutine to avoid blocking
	go func() {
		if err := p.sendKillFeedEmbed(event); err != nil {
			p.apis.LogAPI.Error("Failed to send Discord embed for kill feed", err, map[string]interface{}{
				"victim_name":                event.VictimName,
				"attacker_player_controller": event.AttackerPlayerController,
				"weapon":                     event.Weapon,
			})
		}
	}()

	return nil
}

// sendKillFeedEmbed sends the kill feed as a Discord embed
func (p *DiscordKillFeedPlugin) sendKillFeedEmbed(event *event_manager.LogPlayerWoundedData) error {
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id not configured")
	}

	// Extract attacker name from player controller (fallback if no separate name field)
	attackerName := event.AttackerPlayerController
	attackerSteamID := event.AttackerSteam
	attackerEOSID := event.AttackerEOS

	// Default values for missing data
	if attackerSteamID == "" {
		attackerSteamID = "Unknown"
	}
	if attackerEOSID == "" {
		attackerEOSID = "Unknown"
	}

	victimName := event.VictimName
	if victimName == "" {
		victimName = "Unknown"
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
			Value:  "Unknown", // Victim SteamID not available in this event
			Inline: true,
		},
		{
			Name:   "Victim's EosID",
			Value:  "Unknown", // Victim EOS ID not available in this event
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

	// Add teamkill indicator if this was a teamkill
	title := fmt.Sprintf("KillFeed: %s", attackerName)
	if event.Teamkill {
		title = fmt.Sprintf("KillFeed (TEAMKILL): %s", attackerName)
	}

	embed := &discord.DiscordEmbed{
		Title:     title,
		Color:     p.getIntConfig("color"),
		Fields:    fields,
		Timestamp: func() *time.Time { t := time.Now(); return &t }(),
	}

	if _, err := p.discordAPI.SendEmbed(channelID, embed); err != nil {
		return fmt.Errorf("failed to send Discord embed: %w", err)
	}

	p.apis.LogAPI.Debug("Sent kill feed to Discord", map[string]interface{}{
		"channel_id":     channelID,
		"attacker_name":  attackerName,
		"victim_name":    victimName,
		"weapon":         event.Weapon,
		"teamkill":       event.Teamkill,
		"attacker_steam": attackerSteamID,
	})

	return nil
}

// Helper methods for config access

func (p *DiscordKillFeedPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *DiscordKillFeedPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *DiscordKillFeedPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}
