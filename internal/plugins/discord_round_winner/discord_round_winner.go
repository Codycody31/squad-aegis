package discord_round_winner

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

// DiscordRoundWinnerPlugin sends round winner information to Discord
type DiscordRoundWinnerPlugin struct {
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
		ID:                     "discord_round_winner",
		Name:                   "Discord Round Winner",
		Description:            "The Discord Round Winner plugin will send the round winner to a Discord channel.",
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
			event_manager.EventTypeLogGameEventUnified, // New unified events
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &DiscordRoundWinnerPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *DiscordRoundWinnerPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

func (p *DiscordRoundWinnerPlugin) GetCommands() []plugin_manager.PluginCommand {
	return []plugin_manager.PluginCommand{}
}

func (p *DiscordRoundWinnerPlugin) ExecuteCommand(commandID string, params map[string]interface{}) (*plugin_manager.CommandResult, error) {
	return nil, fmt.Errorf("no commands available")
}

func (p *DiscordRoundWinnerPlugin) GetCommandExecutionStatus(executionID string) (*plugin_manager.CommandExecutionStatus, error) {
	return nil, fmt.Errorf("no commands available")
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *DiscordRoundWinnerPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
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
func (p *DiscordRoundWinnerPlugin) Start(ctx context.Context) error {
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
func (p *DiscordRoundWinnerPlugin) Stop() error {
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
func (p *DiscordRoundWinnerPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != string(event_manager.EventTypeLogGameEventUnified) {
		return nil // Not interested in this event
	}

	return p.handleNewGame(event)
}

// GetStatus returns the current plugin status
func (p *DiscordRoundWinnerPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *DiscordRoundWinnerPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *DiscordRoundWinnerPlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Discord Round Winner plugin configuration updated", map[string]interface{}{
		"channel_id": config["channel_id"],
		"color":      config["color"],
	})

	return nil
}

// handleNewGame processes new game events
func (p *DiscordRoundWinnerPlugin) handleNewGame(rawEvent *plugin_manager.PluginEvent) error {
	// Handle both old and new event types for backwards compatibility
	var winner, layer string

	if unifiedEvent, ok := rawEvent.Data.(*event_manager.LogGameEventUnifiedData); ok {
		if unifiedEvent.EventType == "ROUND_ENDED" {
			winner = unifiedEvent.Winner
			layer = unifiedEvent.Layer
		} else {
			return nil // Not a new game event
		}
	} else {
		return fmt.Errorf("invalid event data type")
	}

	if err := p.sendRoundWinnerEmbed(winner, layer); err != nil {
		p.apis.LogAPI.Error("Failed to send Discord embed for round winner", err, map[string]interface{}{
			"layer": layer,
		})
	}

	return nil
}

// sendRoundWinnerEmbed sends the round winner as a Discord embed
func (p *DiscordRoundWinnerPlugin) sendRoundWinnerEmbed(winner, layer string) error {
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id not configured")
	}

	embed := &discord.DiscordEmbed{
		Title: "Round Winner",
		Color: p.getIntConfig("color"),
		Fields: []*discord.DiscordEmbedField{
			{
				Name:  "Message",
				Value: fmt.Sprintf("%s won on %s", winner, layer),
			},
		},
		Timestamp: func() *time.Time { t := time.Now(); return &t }(),
	}

	if _, err := p.discordAPI.SendEmbed(channelID, embed); err != nil {
		return fmt.Errorf("failed to send Discord embed: %w", err)
	}

	p.apis.LogAPI.Info("Sent round winner notification to Discord", map[string]interface{}{
		"channel_id": channelID,
		"winner":     winner,
		"layer":      layer,
	})

	return nil
}

// Helper methods for config access

func (p *DiscordRoundWinnerPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *DiscordRoundWinnerPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}
