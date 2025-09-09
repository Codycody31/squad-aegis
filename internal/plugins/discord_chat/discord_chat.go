package discord_chat

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

// DiscordChatPlugin logs in-game chat to Discord
type DiscordChatPlugin struct {
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
		ID:                     "discord_chat",
		Name:                   "Discord Chat",
		Description:            "The Discord Chat plugin will log in-game chat to a Discord channel.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{"discord"},
		LongRunning:            false,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "channel_id",
					Description: "The ID of the channel to log chat messages to.",
					Required:    true,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "",
				},
				{
					Name:        "chat_colors",
					Description: "The color of the embed for each chat type. Map of chat type to color (e.g., {'ChatAll': 16761867}).",
					Required:    false,
					Type:        plug_config_schema.FieldTypeObject,
					Default:     map[string]interface{}{},
				},
				{
					Name:        "color",
					Description: "The default color of the embed if no specific chat color is set.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     16761867, // Orange color
				},
				{
					Name:        "ignore_chats",
					Description: "A list of chat types to ignore (e.g., ['ChatSquad']).",
					Required:    false,
					Type:        plug_config_schema.FieldTypeArray,
					Default:     []interface{}{"ChatSquad"},
				},
				{
					Name:        "enabled",
					Description: "Whether the plugin is enabled.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeRconChatMessage,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &DiscordChatPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *DiscordChatPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *DiscordChatPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
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
func (p *DiscordChatPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	// Check if plugin is enabled
	if !p.getBoolConfig("enabled") {
		p.apis.LogAPI.Info("Discord Chat plugin is disabled", nil)
		return nil
	}

	// Validate channel ID
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id is required but not configured")
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	p.apis.LogAPI.Info("Discord Chat plugin started", map[string]interface{}{
		"channel_id":    channelID,
		"color":         p.getIntConfig("color"),
		"ignore_chats":  p.getArrayConfig("ignore_chats"),
	})

	return nil
}

// Stop gracefully stops the plugin
func (p *DiscordChatPlugin) Stop() error {
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

	p.apis.LogAPI.Info("Discord Chat plugin stopped", nil)

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *DiscordChatPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != "RCON_CHAT_MESSAGE" {
		return nil // Not interested in this event
	}

	return p.handleChatMessage(event)
}

// GetStatus returns the current plugin status
func (p *DiscordChatPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *DiscordChatPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *DiscordChatPlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Discord Chat plugin configuration updated", map[string]interface{}{
		"channel_id":   config["channel_id"],
		"color":        config["color"],
		"ignore_chats": config["ignore_chats"],
		"enabled":      config["enabled"],
	})

	return nil
}

// handleChatMessage processes chat message events
func (p *DiscordChatPlugin) handleChatMessage(rawEvent *plugin_manager.PluginEvent) error {
	if !p.getBoolConfig("enabled") {
		return nil // Plugin is disabled
	}

	event, ok := rawEvent.Data.(*event_manager.RconChatMessageData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	// Check if this chat type should be ignored
	ignoreChats := p.getArrayConfig("ignore_chats")
	for _, ignoredChat := range ignoreChats {
		if chatStr, ok := ignoredChat.(string); ok && chatStr == event.ChatType {
			return nil // This chat type is ignored
		}
	}

	// Send Discord embed in a goroutine to avoid blocking
	go func() {
		if err := p.sendChatEmbed(event); err != nil {
			p.apis.LogAPI.Error("Failed to send Discord embed for chat message", err, map[string]interface{}{
				"player_name": event.PlayerName,
				"chat_type":   event.ChatType,
				"message":     event.Message,
			})
		}
	}()

	return nil
}

// sendChatEmbed sends the chat message as a Discord embed
func (p *DiscordChatPlugin) sendChatEmbed(event *event_manager.RconChatMessageData) error {
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id not configured")
	}

	// Get player info to populate team/squad data
	var teamInfo string = "Unknown"
	var squadInfo string = "Unknown"
	
	// Try to get current player list to find team/squad info
	if players, err := p.apis.ServerAPI.GetPlayers(); err == nil {
		for _, player := range players {
			if player.SteamID == event.SteamID {
				teamInfo = fmt.Sprintf("%d", player.TeamID)
				if player.SquadID > 0 {
					squadInfo = fmt.Sprintf("%d", player.SquadID)
				} else {
					squadInfo = "Unassigned"
				}
				break
			}
		}
	}

	// Get the color for this chat type
	color := p.getIntConfig("color") // Default color
	if chatColors := p.getMapConfig("chat_colors"); chatColors != nil {
		if chatColor, exists := chatColors[event.ChatType]; exists {
			if colorInt, ok := chatColor.(int); ok {
				color = colorInt
			} else if colorFloat, ok := chatColor.(float64); ok {
				color = int(colorFloat)
			}
		}
	}

	embed := &discord.DiscordEmbed{
		Title: event.ChatType,
		Color: color,
		Fields: []*discord.DiscordEmbedField{
			{
				Name:   "Player",
				Value:  event.PlayerName,
				Inline: true,
			},
			{
				Name:   "SteamID",
				Value:  fmt.Sprintf("[%s](https://steamcommunity.com/profiles/%s)", event.SteamID, event.SteamID),
				Inline: true,
			},
			{
				Name:   "EosID",
				Value:  event.EosID,
				Inline: true,
			},
			{
				Name:  "Team & Squad",
				Value: fmt.Sprintf("Team: %s, Squad: %s", teamInfo, squadInfo),
			},
			{
				Name:  "Message",
				Value: event.Message,
			},
		},
		Timestamp: func() *time.Time { t := time.Now(); return &t }(),
	}

	if err := p.discordAPI.SendEmbed(channelID, embed); err != nil {
		return fmt.Errorf("failed to send Discord embed: %w", err)
	}

	p.apis.LogAPI.Debug("Sent chat message to Discord", map[string]interface{}{
		"channel_id":   channelID,
		"player_name":  event.PlayerName,
		"chat_type":    event.ChatType,
		"message":      event.Message,
		"steam_id":     event.SteamID,
	})

	return nil
}

// Helper methods for config access

func (p *DiscordChatPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *DiscordChatPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *DiscordChatPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}

func (p *DiscordChatPlugin) getArrayConfig(key string) []interface{} {
	if value, ok := p.config[key].([]interface{}); ok {
		return value
	}
	return []interface{}{}
}

func (p *DiscordChatPlugin) getMapConfig(key string) map[string]interface{} {
	if value, ok := p.config[key].(map[string]interface{}); ok {
		return value
	}
	return nil
}
