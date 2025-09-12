package discord_squad_created

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

// DiscordSquadCreatedPlugin logs squad creation events to Discord
type DiscordSquadCreatedPlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// Discord connector
	discordAPI discord.DiscordAPI

	// State management
	mu     sync.Mutex
	status plugin_manager.PluginStatus
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "discord_squad_created",
		Name:                   "Discord Squad Created",
		Description:            "The SquadCreated plugin will log Squad Creation events to a Discord channel.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{"discord"},

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "channel_id",
					Description: "The ID of the channel to log Squad Creation events to.",
					Required:    true,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "",
				},
				{
					Name:        "color",
					Description: "The color of the embed.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     16761867, // Light blue color
				},
				{
					Name:        "use_embed",
					Description: "Send message as Embed.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "enabled",
					Description: "Whether the plugin is enabled.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     false, // Matches defaultEnabled: false
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeRconSquadCreated,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &DiscordSquadCreatedPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *DiscordSquadCreatedPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *DiscordSquadCreatedPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
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
func (p *DiscordSquadCreatedPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	// Check if plugin is enabled
	if !p.getBoolConfig("enabled") {
		p.apis.LogAPI.Info("Discord Squad Created plugin is disabled", nil)
		return nil
	}

	// Validate channel ID
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id is required but not configured")
	}

	p.status = plugin_manager.PluginStatusRunning

	p.apis.LogAPI.Info("Discord Squad Created plugin started", map[string]interface{}{
		"channel_id": channelID,
		"use_embed":  p.getBoolConfig("use_embed"),
		"color":      p.getIntConfig("color"),
	})

	return nil
}

// Stop gracefully stops the plugin
func (p *DiscordSquadCreatedPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusStopped {
		return nil // Already stopped
	}

	p.status = plugin_manager.PluginStatusStopped

	p.apis.LogAPI.Info("Discord Squad Created plugin stopped", nil)

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *DiscordSquadCreatedPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != "RCON_SQUAD_CREATED" {
		return nil // Not interested in this event
	}

	return p.handleSquadCreated(event)
}

// GetStatus returns the current plugin status
func (p *DiscordSquadCreatedPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *DiscordSquadCreatedPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *DiscordSquadCreatedPlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Discord Squad Created plugin configuration updated", map[string]interface{}{
		"channel_id": config["channel_id"],
		"use_embed":  config["use_embed"],
		"color":      config["color"],
		"enabled":    config["enabled"],
	})

	return nil
}

// handleSquadCreated processes squad creation events
func (p *DiscordSquadCreatedPlugin) handleSquadCreated(rawEvent *plugin_manager.PluginEvent) error {
	if !p.getBoolConfig("enabled") {
		return nil // Plugin is disabled
	}

	event, ok := rawEvent.Data.(*event_manager.RconSquadCreatedData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id not configured")
	}

	if p.getBoolConfig("use_embed") {
		return p.sendEmbedMessage(channelID, event)
	} else {
		return p.sendTextMessage(channelID, event)
	}
}

// sendEmbedMessage sends the squad creation event as a Discord embed
func (p *DiscordSquadCreatedPlugin) sendEmbedMessage(channelID string, event *event_manager.RconSquadCreatedData) error {
	embed := &discord.DiscordEmbed{
		Title: "Squad Created",
		Color: p.getIntConfig("color"),
		Fields: []*discord.DiscordEmbedField{
			{
				Name:   "Player",
				Value:  event.PlayerName,
				Inline: true,
			},
			{
				Name:   "Team",
				Value:  event.TeamName,
				Inline: true,
			},
			{
				Name:  "Squad Number & Squad Name",
				Value: fmt.Sprintf("%s : %s", event.SquadID, event.SquadName),
			},
		},
		Timestamp: func() *time.Time { t := time.Now(); return &t }(),
	}

	if _, err := p.discordAPI.SendEmbed(channelID, embed); err != nil {
		return fmt.Errorf("failed to send Discord embed: %w", err)
	}

	p.apis.LogAPI.Debug("Sent squad creation embed to Discord", map[string]interface{}{
		"channel_id":  channelID,
		"player_name": event.PlayerName,
		"team_name":   event.TeamName,
		"squad_id":    event.SquadID,
		"squad_name":  event.SquadName,
	})

	return nil
}

// sendTextMessage sends the squad creation event as plain text
func (p *DiscordSquadCreatedPlugin) sendTextMessage(channelID string, event *event_manager.RconSquadCreatedData) error {
	message := fmt.Sprintf("```Player: %s\n created Squad %s : %s\n on %s```",
		event.PlayerName,
		event.SquadID,
		event.SquadName,
		event.TeamName,
	)

	if _, err := p.discordAPI.SendMessage(channelID, message); err != nil {
		return fmt.Errorf("failed to send Discord message: %w", err)
	}

	p.apis.LogAPI.Debug("Sent squad creation message to Discord", map[string]interface{}{
		"channel_id":  channelID,
		"player_name": event.PlayerName,
		"team_name":   event.TeamName,
		"squad_id":    event.SquadID,
		"squad_name":  event.SquadName,
	})

	return nil
}

// Helper methods for config access

func (p *DiscordSquadCreatedPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *DiscordSquadCreatedPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *DiscordSquadCreatedPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}
