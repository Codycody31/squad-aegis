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
				plug_config_schema.NewStringField(
					"channel_id",
					"The ID of the channel to log chat messages to",
					true,
					"",
				),
				plug_config_schema.NewObjectField(
					"chat_colors",
					"The color of the embed for each chat type (map of chat type to color hex value)",
					false,
					[]plug_config_schema.ConfigField{
						plug_config_schema.NewIntField("ChatAll", "Color for all chat messages", false, 16761867),
						plug_config_schema.NewIntField("ChatTeam", "Color for team chat messages", false, 65280),
						plug_config_schema.NewIntField("ChatAdmin", "Color for admin chat messages", false, 16711680),
					},
					map[string]interface{}{},
				),
				plug_config_schema.NewIntField(
					"color",
					"The default color of the embed if no specific chat color is set",
					false,
					16761867,
				),
				{
					Name:        "ignore_chats",
					Description: "A list of chat types to ignore (e.g., ChatSquad, ChatAll)",
					Required:    false,
					Type:        plug_config_schema.FieldTypeArrayString,
					Default:     []interface{}{"ChatSquad"},
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
	})

	return nil
}

// handleChatMessage processes chat message events
func (p *DiscordChatPlugin) handleChatMessage(rawEvent *plugin_manager.PluginEvent) error {
	event, ok := rawEvent.Data.(*event_manager.RconChatMessageData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	// Check if this chat type should be ignored
	ignoreChats := p.getArrayConfig("ignore_chats")
	for _, ignoredChat := range ignoreChats {
		if ignoredChat == event.ChatType {
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

	if _, err := p.discordAPI.SendEmbed(channelID, embed); err != nil {
		return fmt.Errorf("failed to send Discord embed: %w", err)
	}

	p.apis.LogAPI.Debug("Sent chat message to Discord", map[string]interface{}{
		"channel_id":  channelID,
		"player_name": event.PlayerName,
		"chat_type":   event.ChatType,
		"message":     event.Message,
		"steam_id":    event.SteamID,
	})

	return nil
}

// Helper methods for config access

func (p *DiscordChatPlugin) getStringConfig(key string) string {
	return plug_config_schema.GetStringValue(p.config, key)
}

func (p *DiscordChatPlugin) getIntConfig(key string) int {
	return plug_config_schema.GetIntValue(p.config, key)
}

func (p *DiscordChatPlugin) getBoolConfig(key string) bool {
	return plug_config_schema.GetBoolValue(p.config, key)
}

func (p *DiscordChatPlugin) getArrayConfig(key string) []string {
	return plug_config_schema.GetArrayStringValue(p.config, key)
}

func (p *DiscordChatPlugin) getMapConfig(key string) map[string]interface{} {
	if value, ok := p.config[key].(map[string]interface{}); ok {
		return value
	}
	return nil
}
