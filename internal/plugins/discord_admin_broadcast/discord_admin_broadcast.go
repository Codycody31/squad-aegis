package discord_admin_broadcast

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

// DiscordAdminBroadcastPlugin sends admin broadcasts to Discord
type DiscordAdminBroadcastPlugin struct {
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
		ID:                     "discord_admin_broadcast",
		Name:                   "Discord Admin Broadcast",
		Description:            "The Discord Admin Broadcast plugin will send a copy of admin broadcasts made in game to a Discord channel.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{"discord"},
		LongRunning:            false,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "channel_id",
					Description: "The ID of the channel to log admin broadcasts to.",
					Required:    true,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "",
				},
				{
					Name:        "color",
					Description: "The color of the embed.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     16761867, // Orange color
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeLogAdminBroadcast,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &DiscordAdminBroadcastPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *DiscordAdminBroadcastPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

func (p *DiscordAdminBroadcastPlugin) GetCommands() []plugin_manager.PluginCommand {
	return []plugin_manager.PluginCommand{}
}

func (p *DiscordAdminBroadcastPlugin) ExecuteCommand(commandID string, params map[string]interface{}) (*plugin_manager.CommandResult, error) {
	return nil, fmt.Errorf("no commands available")
}

func (p *DiscordAdminBroadcastPlugin) GetCommandExecutionStatus(executionID string) (*plugin_manager.CommandExecutionStatus, error) {
	return nil, fmt.Errorf("no commands available")
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *DiscordAdminBroadcastPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
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
func (p *DiscordAdminBroadcastPlugin) Start(ctx context.Context) error {
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

	p.apis.LogAPI.Info("Discord Admin Broadcast plugin started", map[string]interface{}{
		"channel_id": channelID,
		"color":      p.getIntConfig("color"),
	})

	return nil
}

// Stop gracefully stops the plugin
func (p *DiscordAdminBroadcastPlugin) Stop() error {
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

	p.apis.LogAPI.Info("Discord Admin Broadcast plugin stopped", nil)

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *DiscordAdminBroadcastPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != "LOG_ADMIN_BROADCAST" {
		return nil // Not interested in this event
	}

	return p.handleAdminBroadcast(event)
}

// GetStatus returns the current plugin status
func (p *DiscordAdminBroadcastPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *DiscordAdminBroadcastPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *DiscordAdminBroadcastPlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Discord Admin Broadcast plugin configuration updated", map[string]interface{}{
		"channel_id": config["channel_id"],
		"color":      config["color"],
	})

	return nil
}

// handleAdminBroadcast processes admin broadcast events
func (p *DiscordAdminBroadcastPlugin) handleAdminBroadcast(rawEvent *plugin_manager.PluginEvent) error {
	event, ok := rawEvent.Data.(*event_manager.LogAdminBroadcastData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	// Send Discord embed in a goroutine to avoid blocking
	go func() {
		if err := p.sendDiscordEmbed(event); err != nil {
			p.apis.LogAPI.Error("Failed to send Discord embed for admin broadcast", err, map[string]interface{}{
				"message": event.Message,
			})
		}
	}()

	return nil
}

// sendDiscordEmbed sends the admin broadcast as a Discord embed
func (p *DiscordAdminBroadcastPlugin) sendDiscordEmbed(event *event_manager.LogAdminBroadcastData) error {
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id not configured")
	}

	embed := &discord.DiscordEmbed{
		Title: "Admin Broadcast",
		Color: p.getIntConfig("color"),
		Fields: []*discord.DiscordEmbedField{
			{
				Name:  "Message",
				Value: event.Message,
			},
		},
		Timestamp: func() *time.Time { t := time.Now(); return &t }(),
	}

	if _, err := p.discordAPI.SendEmbed(channelID, embed); err != nil {
		return fmt.Errorf("failed to send Discord embed: %w", err)
	}

	p.apis.LogAPI.Info("Sent admin broadcast to Discord", map[string]interface{}{
		"channel_id": channelID,
		"message":    event.Message,
	})

	return nil
}

// Helper methods for config access

func (p *DiscordAdminBroadcastPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *DiscordAdminBroadcastPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *DiscordAdminBroadcastPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}
