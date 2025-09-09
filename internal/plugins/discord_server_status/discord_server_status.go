package discord_server_status

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/connectors/discord"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// DiscordServerStatusPlugin provides real-time server status updates in Discord
type DiscordServerStatusPlugin struct {
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

	// Update intervals
	statusTicker *time.Ticker
}

const COPYRIGHT_MESSAGE = "Squad Aegis Server Monitor"

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "discord_server_status",
		Name:                   "Discord Server Status",
		Description:            "The Discord Server Status plugin can be used to get the server status in Discord with automatic updates.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{"discord"},
		LongRunning:            true, // This plugin runs continuously with timers

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "channel_id",
					Description: "The ID of the channel to send server status updates to.",
					Required:    true,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "",
				},
				{
					Name:        "command",
					Description: "Command name to get message.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "!status",
				},
				{
					Name:        "update_interval_seconds",
					Description: "How frequently to update the status in Discord (in seconds).",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     60, // 60 seconds
				},
				{
					Name:        "set_bot_status",
					Description: "Whether to update the bot's status with server information.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
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
			event_manager.EventTypeRconChatMessage, // Listen for commands
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &DiscordServerStatusPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *DiscordServerStatusPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *DiscordServerStatusPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
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
func (p *DiscordServerStatusPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	// Check if plugin is enabled
	if !p.getBoolConfig("enabled") {
		p.apis.LogAPI.Info("Discord Server Status plugin is disabled", nil)
		return nil
	}

	// Validate channel ID
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id is required but not configured")
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	// Start periodic status updates
	updateInterval := time.Duration(p.getIntConfig("update_interval_seconds")) * time.Second
	p.statusTicker = time.NewTicker(updateInterval)

	// Start goroutine for periodic status messages
	go p.periodicStatusUpdate()

	p.apis.LogAPI.Info("Discord Server Status plugin started", map[string]interface{}{
		"channel_id":              channelID,
		"command":                 p.getStringConfig("command"),
		"update_interval_seconds": p.getIntConfig("update_interval_seconds"),
	})

	return nil
}

// Stop gracefully stops the plugin
func (p *DiscordServerStatusPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusStopped {
		return nil // Already stopped
	}

	p.status = plugin_manager.PluginStatusStopping

	// Stop tickers
	if p.updateTicker != nil {
		p.updateTicker.Stop()
	}
	if p.statusUpdateTicker != nil {
		p.statusUpdateTicker.Stop()
	}

	if p.cancel != nil {
		p.cancel()
	}

	p.status = plugin_manager.PluginStatusStopped

	p.apis.LogAPI.Info("Discord Server Status plugin stopped", nil)

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *DiscordServerStatusPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != "RCON_CHAT_MESSAGE" {
		return nil // Not interested in this event
	}

	return p.handleChatMessage(event)
}

// GetStatus returns the current plugin status
func (p *DiscordServerStatusPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *DiscordServerStatusPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *DiscordServerStatusPlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Discord Server Status plugin configuration updated", map[string]interface{}{
		"channel_id":              config["channel_id"],
		"command":                 config["command"],
		"update_interval_seconds": config["update_interval_seconds"],
		"set_bot_status":          config["set_bot_status"],
		"enabled":                 config["enabled"],
	})

	return nil
}

// handleChatMessage processes chat message events for commands
func (p *DiscordServerStatusPlugin) handleChatMessage(rawEvent *plugin_manager.PluginEvent) error {
	if !p.getBoolConfig("enabled") {
		return nil // Plugin is disabled
	}

	event, ok := rawEvent.Data.(*event_manager.RconChatMessageData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	command := p.getStringConfig("command")
	if event.Message == command {
		// Send server status in response to command
		go func() {
			if err := p.sendServerStatusMessage(); err != nil {
				p.apis.LogAPI.Error("Failed to send server status message", err, map[string]interface{}{
					"player_name": event.PlayerName,
					"command":     command,
				})
			}
		}()
	}

	return nil
}

// periodicMessageUpdate runs the periodic message update loop
func (p *DiscordServerStatusPlugin) periodicMessageUpdate() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.updateTicker.C:
			if err := p.updateTrackedMessages(); err != nil {
				p.apis.LogAPI.Error("Failed to update tracked messages", err, nil)
			}
		}
	}
}

// periodicStatusUpdate runs the periodic bot status update loop
func (p *DiscordServerStatusPlugin) periodicStatusUpdate() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.statusUpdateTicker.C:
			if err := p.updateBotStatus(); err != nil {
				p.apis.LogAPI.Error("Failed to update bot status", err, nil)
			}
		}
	}
}

// sendServerStatusMessage sends a new server status message
func (p *DiscordServerStatusPlugin) sendServerStatusMessage() error {
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id not configured")
	}

	embed, err := p.generateServerStatusEmbed()
	if err != nil {
		return fmt.Errorf("failed to generate server status embed: %w", err)
	}

	message, err := p.discordAPI.SendEmbed(channelID, embed)
	if err != nil {
		return fmt.Errorf("failed to send Discord embed: %w", err)
	}

	// Track this message for future updates
	p.mu.Lock()
	p.trackedMessages[channelID] = message
	p.mu.Unlock()

	p.apis.LogAPI.Debug("Sent server status message to Discord", map[string]interface{}{
		"channel_id": channelID,
	})

	return nil
}

// updateTrackedMessages updates all tracked messages
func (p *DiscordServerStatusPlugin) updateTrackedMessages() error {
	p.mu.Lock()
	trackedMessages := make(map[string]*discord.DiscordMessage)
	for k, v := range p.trackedMessages {
		trackedMessages[k] = v
	}
	p.mu.Unlock()

	for channelID, message := range trackedMessages {
		embed, err := p.generateServerStatusEmbed()
		if err != nil {
			p.apis.LogAPI.Error("Failed to generate server status embed for update", err, map[string]interface{}{
				"channel_id": channelID,
			})
			continue
		}

		if err := p.discordAPI.UpdateMessage(message, embed); err != nil {
			p.apis.LogAPI.Error("Failed to update Discord message", err, map[string]interface{}{
				"channel_id": channelID,
				"message_id": message.ID,
			})
			// Remove failed message from tracking
			p.mu.Lock()
			delete(p.trackedMessages, channelID)
			p.mu.Unlock()
		}
	}

	return nil
}

// updateBotStatus updates the Discord bot's status
func (p *DiscordServerStatusPlugin) updateBotStatus() error {
	if !p.getBoolConfig("set_bot_status") {
		return nil // Bot status updates disabled
	}

	// Get server information
	serverInfo, err := p.apis.ServerAPI.GetServerInfo()
	if err != nil {
		return fmt.Errorf("failed to get server info: %w", err)
	}

	players, err := p.apis.ServerAPI.GetPlayers()
	if err != nil {
		return fmt.Errorf("failed to get players: %w", err)
	}

	playerCount := len(players)
	statusText := fmt.Sprintf("(%d/%d) %s", playerCount, serverInfo.MaxPlayers, serverInfo.CurrentMap)

	// Update bot status using Discord API
	if err := p.discordAPI.SetBotStatus(statusText); err != nil {
		return fmt.Errorf("failed to set bot status: %w", err)
	}

	return nil
}

// generateServerStatusEmbed creates the server status embed
func (p *DiscordServerStatusPlugin) generateServerStatusEmbed() (*discord.DiscordEmbed, error) {
	// Get server information
	serverInfo, err := p.apis.ServerAPI.GetServerInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get server info: %w", err)
	}

	players, err := p.apis.ServerAPI.GetPlayers()
	if err != nil {
		return nil, fmt.Errorf("failed to get players: %w", err)
	}

	playerCount := len(players)

	// Build player count string
	playersField := fmt.Sprintf("%d / %d", playerCount, serverInfo.MaxPlayers)

	// Calculate color based on player ratio (red -> yellow -> green)
	ratio := float64(playerCount) / float64(serverInfo.MaxPlayers)
	clampedRatio := math.Min(1.0, math.Max(0.0, ratio))
	color := p.calculateGradientColor(clampedRatio)

	// Get current and next layer information
	currentLayer := serverInfo.CurrentMap
	if currentLayer == "" {
		currentLayer = "Unknown"
	}

	nextLayer := "Unknown" // We don't have next layer info in our ServerInfo struct

	embed := &discord.DiscordEmbed{
		Title: serverInfo.Name,
		Fields: []*discord.DiscordEmbedField{
			{
				Name:  "Players",
				Value: playersField,
			},
			{
				Name:   "Current Layer",
				Value:  fmt.Sprintf("```%s```", currentLayer),
				Inline: true,
			},
			{
				Name:   "Next Layer",
				Value:  fmt.Sprintf("```%s```", nextLayer),
				Inline: true,
			},
		},
		Color: color,
		Footer: &discord.DiscordEmbedFooter{
			Text: COPYRIGHT_MESSAGE,
		},
		Timestamp: func() *time.Time { t := time.Now(); return &t }(),
	}

	return embed, nil
}

// calculateGradientColor calculates a gradient color from red to yellow to green
func (p *DiscordServerStatusPlugin) calculateGradientColor(ratio float64) int {
	// Clamp ratio between 0 and 1
	ratio = math.Min(1.0, math.Max(0.0, ratio))

	var r, g, b int

	if ratio < 0.5 {
		// Red to Yellow (0.0 -> 0.5)
		localRatio := ratio * 2.0 // Scale to 0-1
		r = 255
		g = int(255 * localRatio)
		b = 0
	} else {
		// Yellow to Green (0.5 -> 1.0)
		localRatio := (ratio - 0.5) * 2.0 // Scale to 0-1
		r = int(255 * (1.0 - localRatio))
		g = 255
		b = 0
	}

	// Convert RGB to hex color integer
	return (r << 16) | (g << 8) | b
}

// Helper methods for config access

func (p *DiscordServerStatusPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *DiscordServerStatusPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *DiscordServerStatusPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}
