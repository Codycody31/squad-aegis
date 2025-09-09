package discord_round_ended

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/connectors/discord"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// DiscordRoundEndedPlugin sends round end information to Discord
type DiscordRoundEndedPlugin struct {
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
		ID:                     "discord_round_ended",
		Name:                   "Discord Round Ended",
		Description:            "The Discord Round Ended plugin will send the round winner to a Discord channel.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{"discord"},
		LongRunning:            false,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "channel_id",
					Description: "The ID of the channel to log round end events to.",
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
			event_manager.EventTypeLogRoundEnded,       // Keep for backwards compatibility
			event_manager.EventTypeLogGameEventUnified, // New unified events
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &DiscordRoundEndedPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *DiscordRoundEndedPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *DiscordRoundEndedPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
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
func (p *DiscordRoundEndedPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	// Check if plugin is enabled
	if !p.getBoolConfig("enabled") {
		p.apis.LogAPI.Info("Discord Round Ended plugin is disabled", nil)
		return nil
	}

	// Validate channel ID
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id is required but not configured")
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	p.apis.LogAPI.Info("Discord Round Ended plugin started", map[string]interface{}{
		"channel_id": channelID,
		"color":      p.getIntConfig("color"),
	})

	return nil
}

// Stop gracefully stops the plugin
func (p *DiscordRoundEndedPlugin) Stop() error {
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

	p.apis.LogAPI.Info("Discord Round Ended plugin stopped", nil)

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *DiscordRoundEndedPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != "LOG_ROUND_ENDED" {
		return nil // Not interested in this event
	}

	return p.handleRoundEnded(event)
}

// GetStatus returns the current plugin status
func (p *DiscordRoundEndedPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *DiscordRoundEndedPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *DiscordRoundEndedPlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Discord Round Ended plugin configuration updated", map[string]interface{}{
		"channel_id": config["channel_id"],
		"color":      config["color"],
		"enabled":    config["enabled"],
	})

	return nil
}

// handleRoundEnded processes round ended events
func (p *DiscordRoundEndedPlugin) handleRoundEnded(rawEvent *plugin_manager.PluginEvent) error {
	if !p.getBoolConfig("enabled") {
		return nil // Plugin is disabled
	}

	// Handle both old and new event types for backwards compatibility
	var eventData *roundEndedEventData

	if unifiedEvent, ok := rawEvent.Data.(*event_manager.LogGameEventUnifiedData); ok {
		if unifiedEvent.EventType == "ROUND_ENDED" {
			eventData = &roundEndedEventData{
				Winner:     unifiedEvent.Winner,
				Layer:      unifiedEvent.Layer,
				WinnerData: unifiedEvent.WinnerData,
				LoserData:  unifiedEvent.LoserData,
			}
		} else {
			return nil // Not a round ended event
		}
	} else if oldEvent, ok := rawEvent.Data.(*event_manager.LogRoundEndedData); ok {
		// Legacy event format
		eventData = &roundEndedEventData{
			Winner: oldEvent.Winner,
			Layer:  oldEvent.Layer,
		}
		// Convert legacy winner/loser data if present
		if oldEvent.WinnerData != nil {
			if winnerJSON, err := json.Marshal(oldEvent.WinnerData); err == nil {
				eventData.WinnerData = string(winnerJSON)
			}
		}
		if oldEvent.LoserData != nil {
			if loserJSON, err := json.Marshal(oldEvent.LoserData); err == nil {
				eventData.LoserData = string(loserJSON)
			}
		}
	} else {
		return fmt.Errorf("invalid event data type")
	}

	// Send Discord embed in a goroutine to avoid blocking
	go func() {
		if err := p.sendRoundEndedEmbed(eventData); err != nil {
			p.apis.LogAPI.Error("Failed to send Discord embed for round ended", err, map[string]interface{}{
				"winner": eventData.Winner,
			})
		}
	}()

	return nil
}

// roundEndedEventData represents normalized round ended event data
type roundEndedEventData struct {
	Winner     string
	Layer      string
	WinnerData string
	LoserData  string
}

// sendRoundEndedEmbed sends the round ended as a Discord embed
func (p *DiscordRoundEndedPlugin) sendRoundEndedEmbed(event *roundEndedEventData) error {
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id not configured")
	}

	// Check if this was a draw (no winner)
	if event.Winner == "" {
		embed := &discord.DiscordEmbed{
			Title:       "Round Ended",
			Description: "This match ended in a Draw",
			Color:       p.getIntConfig("color"),
			Timestamp:   func() *time.Time { t := time.Now(); return &t }(),
		}

		if err := p.discordAPI.SendEmbed(channelID, embed); err != nil {
			return fmt.Errorf("failed to send Discord embed: %w", err)
		}

		p.apis.LogAPI.Info("Sent round ended (draw) notification to Discord", map[string]interface{}{
			"channel_id": channelID,
		})

		return nil
	}

	// For matches with a winner, construct detailed embed
	description := fmt.Sprintf("Round ended on %s", event.Layer)
	if event.Layer == "" {
		description = "Round ended"
	}

	fields := []*discord.DiscordEmbedField{
		{
			Name:  "Winner",
			Value: event.Winner,
		},
	}

	// Extract winner information from WinnerData JSON
	if event.WinnerData != "" {
		var winnerData map[string]interface{}
		if err := json.Unmarshal([]byte(event.WinnerData), &winnerData); err == nil {
			if team, ok := winnerData["team"].(string); ok && team != "" {
				if subfaction, ok := winnerData["subfaction"].(string); ok && subfaction != "" {
					if faction, ok := winnerData["faction"].(string); ok && faction != "" {
						if tickets, ok := winnerData["tickets"].(string); ok && tickets != "" {
							fields = append(fields, &discord.DiscordEmbedField{
								Name:  fmt.Sprintf("Team %s Won", team),
								Value: fmt.Sprintf("%s\n%s\nwon with %s tickets.", subfaction, faction, tickets),
							})
						}
					}
				}
			}
		}
	}

	// Extract loser information from LoserData JSON
	if event.LoserData != "" {
		var loserData map[string]interface{}
		if err := json.Unmarshal([]byte(event.LoserData), &loserData); err == nil {
			if team, ok := loserData["team"].(string); ok && team != "" {
				if subfaction, ok := loserData["subfaction"].(string); ok && subfaction != "" {
					if faction, ok := loserData["faction"].(string); ok && faction != "" {
						if tickets, ok := loserData["tickets"].(string); ok && tickets != "" {
							fields = append(fields, &discord.DiscordEmbedField{
								Name:  fmt.Sprintf("Team %s Lost", team),
								Value: fmt.Sprintf("%s\n%s\nlost with %s tickets.", subfaction, faction, tickets),
							})
						}
					}
				}
			}
		}
	}

	// Calculate ticket difference if both are available
	if event.WinnerData != "" && event.LoserData != "" {
		var winnerData, loserData map[string]interface{}
		if err1 := json.Unmarshal([]byte(event.WinnerData), &winnerData); err1 == nil {
			if err2 := json.Unmarshal([]byte(event.LoserData), &loserData); err2 == nil {
				if winnerTicketsStr, ok := winnerData["tickets"].(string); ok {
					if loserTicketsStr, ok := loserData["tickets"].(string); ok {
						if winnerTickets, err1 := strconv.Atoi(winnerTicketsStr); err1 == nil {
							if loserTickets, err2 := strconv.Atoi(loserTicketsStr); err2 == nil {
								ticketDiff := winnerTickets - loserTickets
								fields = append(fields, &discord.DiscordEmbedField{
									Name:  "Ticket Difference",
									Value: fmt.Sprintf("%d", ticketDiff),
								})
							}
						}
					}
				}
			}
		}
	}

	embed := &discord.DiscordEmbed{
		Title:       "Round Ended",
		Description: description,
		Color:       p.getIntConfig("color"),
		Fields:      fields,
		Timestamp:   func() *time.Time { t := time.Now(); return &t }(),
	}

	if err := p.discordAPI.SendEmbed(channelID, embed); err != nil {
		return fmt.Errorf("failed to send Discord embed: %w", err)
	}

	// Extract data for logging
	winnerTeam := ""
	winnerTickets := ""
	loserTeam := ""
	loserTickets := ""

	if event.WinnerData != "" {
		var winnerData map[string]interface{}
		if err := json.Unmarshal([]byte(event.WinnerData), &winnerData); err == nil {
			if team, ok := winnerData["team"].(string); ok {
				winnerTeam = team
			}
			if tickets, ok := winnerData["tickets"].(string); ok {
				winnerTickets = tickets
			}
		}
	}

	if event.LoserData != "" {
		var loserData map[string]interface{}
		if err := json.Unmarshal([]byte(event.LoserData), &loserData); err == nil {
			if team, ok := loserData["team"].(string); ok {
				loserTeam = team
			}
			if tickets, ok := loserData["tickets"].(string); ok {
				loserTickets = tickets
			}
		}
	}

	p.apis.LogAPI.Info("Sent round ended notification to Discord", map[string]interface{}{
		"channel_id":     channelID,
		"winner":         event.Winner,
		"layer":          event.Layer,
		"winner_team":    winnerTeam,
		"winner_tickets": winnerTickets,
		"loser_team":     loserTeam,
		"loser_tickets":  loserTickets,
	})

	return nil
}

// Helper methods for config access

func (p *DiscordRoundEndedPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *DiscordRoundEndedPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *DiscordRoundEndedPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}
