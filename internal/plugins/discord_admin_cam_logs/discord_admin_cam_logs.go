package discord_admin_cam_logs

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

// AdminCamSession tracks an admin's camera session
type AdminCamSession struct {
	PlayerName string
	SteamID    string
	EosID      string
	StartTime  time.Time
}

// DiscordAdminCamLogsPlugin logs admin camera usage to Discord
type DiscordAdminCamLogsPlugin struct {
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

	// Track admin camera sessions
	adminsInCam map[string]*AdminCamSession
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "discord_admin_cam_logs",
		Name:                   "Discord Admin Camera Logs",
		Description:            "The Discord Admin Camera Logs plugin will log in game admin camera usage to a Discord channel.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{"discord"},
		LongRunning:            false,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "channel_id",
					Description: "The ID of the channel to log admin camera usage to.",
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
			event_manager.EventTypeRconPossessedAdminCamera,
			event_manager.EventTypeRconUnpossessedAdminCamera,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &DiscordAdminCamLogsPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *DiscordAdminCamLogsPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

func (p *DiscordAdminCamLogsPlugin) GetCommands() []plugin_manager.PluginCommand {
	return []plugin_manager.PluginCommand{}
}

func (p *DiscordAdminCamLogsPlugin) ExecuteCommand(commandID string, params map[string]interface{}) (*plugin_manager.CommandResult, error) {
	return nil, fmt.Errorf("no commands available")
}

func (p *DiscordAdminCamLogsPlugin) GetCommandExecutionStatus(executionID string) (*plugin_manager.CommandExecutionStatus, error) {
	return nil, fmt.Errorf("no commands available")
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *DiscordAdminCamLogsPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	p.apis = apis
	p.status = plugin_manager.PluginStatusStopped
	p.adminsInCam = make(map[string]*AdminCamSession)

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
func (p *DiscordAdminCamLogsPlugin) Start(ctx context.Context) error {
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

	p.apis.LogAPI.Info("Discord Admin Camera Logs plugin started", map[string]interface{}{
		"channel_id": channelID,
		"color":      p.getIntConfig("color"),
	})

	return nil
}

// Stop gracefully stops the plugin
func (p *DiscordAdminCamLogsPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusStopped {
		return nil // Already stopped
	}

	p.status = plugin_manager.PluginStatusStopping

	if p.cancel != nil {
		p.cancel()
	}

	// Clear admin camera sessions
	p.adminsInCam = make(map[string]*AdminCamSession)

	p.status = plugin_manager.PluginStatusStopped

	p.apis.LogAPI.Info("Discord Admin Camera Logs plugin stopped", nil)

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *DiscordAdminCamLogsPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	switch event.Type {
	case "RCON_POSSESSED_ADMIN_CAMERA":
		return p.handleAdminCameraEntry(event)
	case "RCON_UNPOSSESSED_ADMIN_CAMERA":
		return p.handleAdminCameraExit(event)
	default:
		return nil // Not interested in this event
	}
}

// GetStatus returns the current plugin status
func (p *DiscordAdminCamLogsPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *DiscordAdminCamLogsPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *DiscordAdminCamLogsPlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Discord Admin Camera Logs plugin configuration updated", map[string]interface{}{
		"channel_id": config["channel_id"],
		"color":      config["color"],
	})

	return nil
}

// handleAdminCameraEntry processes admin camera possession events
func (p *DiscordAdminCamLogsPlugin) handleAdminCameraEntry(rawEvent *plugin_manager.PluginEvent) error {
	event, ok := rawEvent.Data.(*event_manager.RconAdminCameraData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	// Track the admin camera session
	p.mu.Lock()
	p.adminsInCam[event.EosID] = &AdminCamSession{
		PlayerName: event.AdminName,
		SteamID:    event.SteamID,
		EosID:      event.EosID,
		StartTime:  time.Now(),
	}
	p.mu.Unlock()

	// Send Discord embed in a goroutine to avoid blocking
	go func() {
		if err := p.sendAdminCameraEntryEmbed(event); err != nil {
			p.apis.LogAPI.Error("Failed to send Discord embed for admin camera entry", err, map[string]interface{}{
				"admin_name": event.AdminName,
				"steam_id":   event.SteamID,
			})
		}
	}()

	return nil
}

// handleAdminCameraExit processes admin camera unpossession events
func (p *DiscordAdminCamLogsPlugin) handleAdminCameraExit(rawEvent *plugin_manager.PluginEvent) error {
	event, ok := rawEvent.Data.(*event_manager.RconAdminCameraData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	// Get the admin camera session
	p.mu.Lock()
	session, exists := p.adminsInCam[event.EosID]
	if exists {
		delete(p.adminsInCam, event.EosID)
	}
	p.mu.Unlock()

	// Calculate duration if we have a session
	var duration time.Duration
	if exists {
		duration = time.Since(session.StartTime)
	}

	// Send Discord embed in a goroutine to avoid blocking
	go func() {
		if err := p.sendAdminCameraExitEmbed(event, duration); err != nil {
			p.apis.LogAPI.Error("Failed to send Discord embed for admin camera exit", err, map[string]interface{}{
				"admin_name": event.AdminName,
				"steam_id":   event.SteamID,
			})
		}
	}()

	return nil
}

// sendAdminCameraEntryEmbed sends the admin camera entry as a Discord embed
func (p *DiscordAdminCamLogsPlugin) sendAdminCameraEntryEmbed(event *event_manager.RconAdminCameraData) error {
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id not configured")
	}

	embed := &discord.DiscordEmbed{
		Title: "Admin Entered Admin Camera",
		Color: p.getIntConfig("color"),
		Fields: []*discord.DiscordEmbedField{
			{
				Name:   "Admin's Name",
				Value:  event.AdminName,
				Inline: true,
			},
			{
				Name:   "Admin's SteamID",
				Value:  fmt.Sprintf("[%s](https://steamcommunity.com/profiles/%s)", event.SteamID, event.SteamID),
				Inline: true,
			},
			{
				Name:   "Admin's EosID",
				Value:  event.EosID,
				Inline: true,
			},
		},
		Timestamp: func() *time.Time { t := time.Now(); return &t }(),
	}

	if _, err := p.discordAPI.SendEmbed(channelID, embed); err != nil {
		return fmt.Errorf("failed to send Discord embed: %w", err)
	}

	p.apis.LogAPI.Info("Sent admin camera entry log to Discord", map[string]interface{}{
		"channel_id": channelID,
		"admin_name": event.AdminName,
		"steam_id":   event.SteamID,
		"eos_id":     event.EosID,
	})

	return nil
}

// sendAdminCameraExitEmbed sends the admin camera exit as a Discord embed
func (p *DiscordAdminCamLogsPlugin) sendAdminCameraExitEmbed(event *event_manager.RconAdminCameraData, duration time.Duration) error {
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id not configured")
	}

	fields := []*discord.DiscordEmbedField{
		{
			Name:   "Admin's Name",
			Value:  event.AdminName,
			Inline: true,
		},
		{
			Name:   "Admin's SteamID",
			Value:  fmt.Sprintf("[%s](https://steamcommunity.com/profiles/%s)", event.SteamID, event.SteamID),
			Inline: true,
		},
		{
			Name:   "Admin's EosID",
			Value:  event.EosID,
			Inline: true,
		},
	}

	// Add duration field if we tracked the session
	if duration > 0 {
		fields = append(fields, &discord.DiscordEmbedField{
			Name:  "Time in Admin Camera",
			Value: fmt.Sprintf("%.1f mins", duration.Minutes()),
		})
	}

	embed := &discord.DiscordEmbed{
		Title:     "Admin Left Admin Camera",
		Color:     p.getIntConfig("color"),
		Fields:    fields,
		Timestamp: func() *time.Time { t := time.Now(); return &t }(),
	}

	if _, err := p.discordAPI.SendEmbed(channelID, embed); err != nil {
		return fmt.Errorf("failed to send Discord embed: %w", err)
	}

	p.apis.LogAPI.Info("Sent admin camera exit log to Discord", map[string]interface{}{
		"channel_id":    channelID,
		"admin_name":    event.AdminName,
		"steam_id":      event.SteamID,
		"eos_id":        event.EosID,
		"duration_mins": duration.Minutes(),
	})

	return nil
}

// Helper methods for config access

func (p *DiscordAdminCamLogsPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *DiscordAdminCamLogsPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}
