package discord_round_winner

import (
	"context"
	"encoding/json"
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
			event_manager.EventTypeLogNewGame,          // Keep for backwards compatibility
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

	p.apis.LogAPI.Info("Discord Round Winner plugin stopped", nil)

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *DiscordRoundWinnerPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != "LOG_NEW_GAME" {
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
	var eventData *newGameEventData

	if unifiedEvent, ok := rawEvent.Data.(*event_manager.LogGameEventUnifiedData); ok {
		if unifiedEvent.EventType == "NEW_GAME" {
			eventData = &newGameEventData{
				Team:           unifiedEvent.Team,
				Subfaction:     unifiedEvent.Subfaction,
				Faction:        unifiedEvent.Faction,
				Action:         unifiedEvent.Action,
				Tickets:        unifiedEvent.Tickets,
				Layer:          unifiedEvent.Layer,
				Level:          unifiedEvent.Level,
				DLC:            unifiedEvent.DLC,
				MapClassname:   unifiedEvent.MapClassname,
				LayerClassname: unifiedEvent.LayerClassname,
			}

			// Parse metadata if available
			if unifiedEvent.Metadata != "" {
				var metadata map[string]interface{}
				if err := json.Unmarshal([]byte(unifiedEvent.Metadata), &metadata); err == nil {
					// Override with metadata values if they exist
					if team, ok := metadata["team"].(string); ok {
						eventData.Team = team
					}
					if subfaction, ok := metadata["subfaction"].(string); ok {
						eventData.Subfaction = subfaction
					}
					if faction, ok := metadata["faction"].(string); ok {
						eventData.Faction = faction
					}
					if action, ok := metadata["action"].(string); ok {
						eventData.Action = action
					}
					if tickets, ok := metadata["tickets"].(string); ok {
						eventData.Tickets = tickets
					}
					if layer, ok := metadata["layer"].(string); ok {
						eventData.Layer = layer
					}
					if level, ok := metadata["level"].(string); ok {
						eventData.Level = level
					}
				}
			}
		} else {
			return nil // Not a new game event
		}
	} else {
		return fmt.Errorf("invalid event data type")
	}

	if err := p.sendRoundWinnerEmbed(eventData); err != nil {
		p.apis.LogAPI.Error("Failed to send Discord embed for round winner", err, map[string]interface{}{
			"layer": eventData.Layer,
		})
	}

	return nil
}

// newGameEventData represents normalized new game event data
type newGameEventData struct {
	Team           string
	Subfaction     string
	Faction        string
	Action         string
	Tickets        string
	Layer          string
	Level          string
	DLC            string
	MapClassname   string
	LayerClassname string
}

// sendRoundWinnerEmbed sends the round winner as a Discord embed
func (p *DiscordRoundWinnerPlugin) sendRoundWinnerEmbed(event *newGameEventData) error {
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id not configured")
	}

	// Get current layer information
	currentLayer := "Unknown"
	if event.Layer != "" {
		currentLayer = event.Layer
	}

	// Construct winner message based on available event data
	var message string
	if event.Team != "" && event.Action == "won" {
		// Format: "{Team} {Faction} won on {Layer}"
		faction := event.Faction
		if faction == "" {
			faction = "faction"
		}
		message = fmt.Sprintf("%s %s won on %s.", event.Team, faction, currentLayer)

		// Add ticket information if available
		if event.Tickets != "" {
			message = fmt.Sprintf("%s %s won on %s with %s tickets remaining.",
				event.Team, faction, currentLayer, event.Tickets)
		}
	} else {
		// Fallback message when detailed winner info is not available
		message = fmt.Sprintf("New game started on %s.", currentLayer)
	}

	embed := &discord.DiscordEmbed{
		Title: "Round Winner",
		Color: p.getIntConfig("color"),
		Fields: []*discord.DiscordEmbedField{
			{
				Name:  "Message",
				Value: message,
			},
		},
		Timestamp: func() *time.Time { t := time.Now(); return &t }(),
	}

	if _, err := p.discordAPI.SendEmbed(channelID, embed); err != nil {
		return fmt.Errorf("failed to send Discord embed: %w", err)
	}

	p.apis.LogAPI.Info("Sent round winner notification to Discord", map[string]interface{}{
		"channel_id": channelID,
		"layer":      currentLayer,
		"team":       event.Team,
		"faction":    event.Faction,
		"action":     event.Action,
		"tickets":    event.Tickets,
		"message":    message,
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
