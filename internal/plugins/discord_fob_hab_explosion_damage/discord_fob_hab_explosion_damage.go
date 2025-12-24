package discord_fob_hab_explosion_damage

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/connectors/discord"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// DiscordFOBHABExplosionDamagePlugin logs FOB/HAB explosion damage to Discord
type DiscordFOBHABExplosionDamagePlugin struct {
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

	// Compiled regex patterns for performance
	fobHabRegex     *regexp.Regexp
	deployableRegex *regexp.Regexp
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "discord_fob_hab_explosion_damage",
		Name:                   "Discord FOB/HAB Explosion Damage",
		Description:            "The Discord FOB/HAB Explosion Damage plugin logs damage done to FOBs and HABs by explosions to help identify engineers blowing up friendly FOBs and HABs.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{"discord"},
		LongRunning:            false,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "channel_id",
					Description: "The ID of the channel to log FOB/HAB explosion damage to.",
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
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeLogDeployableDamaged,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &DiscordFOBHABExplosionDamagePlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *DiscordFOBHABExplosionDamagePlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

func (p *DiscordFOBHABExplosionDamagePlugin) GetCommands() []plugin_manager.PluginCommand {
	return []plugin_manager.PluginCommand{}
}

func (p *DiscordFOBHABExplosionDamagePlugin) ExecuteCommand(commandID string, params map[string]interface{}) (*plugin_manager.CommandResult, error) {
	return nil, fmt.Errorf("no commands available")
}

func (p *DiscordFOBHABExplosionDamagePlugin) GetCommandExecutionStatus(executionID string) (*plugin_manager.CommandExecutionStatus, error) {
	return nil, fmt.Errorf("no commands available")
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *DiscordFOBHABExplosionDamagePlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	p.apis = apis
	p.status = plugin_manager.PluginStatusStopped

	// Compile regex patterns for performance
	var err error
	p.fobHabRegex, err = regexp.Compile(`(?i)(?:FOBRadio|Hab)_`)
	if err != nil {
		return fmt.Errorf("failed to compile FOB/HAB regex: %w", err)
	}

	p.deployableRegex, err = regexp.Compile(`(?i)_Deployable_`)
	if err != nil {
		return fmt.Errorf("failed to compile deployable regex: %w", err)
	}

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
func (p *DiscordFOBHABExplosionDamagePlugin) Start(ctx context.Context) error {
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
func (p *DiscordFOBHABExplosionDamagePlugin) Stop() error {
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
func (p *DiscordFOBHABExplosionDamagePlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != "LOG_DEPLOYABLE_DAMAGED" {
		return nil // Not interested in this event
	}

	return p.handleDeployableDamaged(event)
}

// GetStatus returns the current plugin status
func (p *DiscordFOBHABExplosionDamagePlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *DiscordFOBHABExplosionDamagePlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *DiscordFOBHABExplosionDamagePlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Discord FOB/HAB Explosion Damage plugin configuration updated", map[string]interface{}{
		"channel_id": config["channel_id"],
		"color":      config["color"],
	})

	return nil
}

// handleDeployableDamaged processes deployable damaged events
func (p *DiscordFOBHABExplosionDamagePlugin) handleDeployableDamaged(rawEvent *plugin_manager.PluginEvent) error {
	event, ok := rawEvent.Data.(*event_manager.LogDeployableDamagedData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	// Check if this is a FOB or HAB
	if !p.fobHabRegex.MatchString(event.Deployable) {
		return nil // Not a FOB or HAB
	}

	// Check if the weapon is a deployable (explosion)
	if !p.deployableRegex.MatchString(event.Weapon) {
		return nil // Not caused by a deployable/explosion
	}

	// Check if we have player information in the PlayerSuffix
	if event.PlayerSuffix == "" {
		return nil // No player information
	}

	// Send Discord embed in a goroutine to avoid blocking
	go func() {
		if err := p.sendFOBHABDamageEmbed(event); err != nil {
			p.apis.LogAPI.Error("Failed to send Discord embed for FOB/HAB explosion damage", err, map[string]interface{}{
				"player_suffix": event.PlayerSuffix,
				"deployable":    event.Deployable,
				"weapon":        event.Weapon,
			})
		}
	}()

	return nil
}

// sendFOBHABDamageEmbed sends the FOB/HAB damage as a Discord embed
func (p *DiscordFOBHABExplosionDamagePlugin) sendFOBHABDamageEmbed(event *event_manager.LogDeployableDamagedData) error {
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id not configured")
	}

	// Try to extract player name from PlayerSuffix
	// PlayerSuffix might contain player controller info or player name
	playerInfo := event.PlayerSuffix
	if playerInfo == "" {
		playerInfo = "Unknown Player"
	}

	embed := &discord.DiscordEmbed{
		Title: fmt.Sprintf("FOB/HAB Explosion Damage: %s", playerInfo),
		Color: p.getIntConfig("color"),
		Fields: []*discord.DiscordEmbedField{
			{
				Name:   "Player Info",
				Value:  playerInfo,
				Inline: true,
			},
			{
				Name:   "Damage",
				Value:  event.Damage,
				Inline: true,
			},
			{
				Name:   "Health Remaining",
				Value:  event.HealthRemaining,
				Inline: true,
			},
			{
				Name:  "Deployable",
				Value: event.Deployable,
			},
			{
				Name:  "Weapon",
				Value: event.Weapon,
			},
			{
				Name:  "Damage Type",
				Value: event.DamageType,
			},
		},
		Timestamp: func() *time.Time { t := time.Now(); return &t }(),
	}

	if _, err := p.discordAPI.SendEmbed(channelID, embed); err != nil {
		return fmt.Errorf("failed to send Discord embed: %w", err)
	}

	p.apis.LogAPI.Info("Sent FOB/HAB explosion damage alert to Discord", map[string]interface{}{
		"channel_id":       channelID,
		"player_info":      playerInfo,
		"deployable":       event.Deployable,
		"weapon":           event.Weapon,
		"damage":           event.Damage,
		"health_remaining": event.HealthRemaining,
	})

	return nil
}

// Helper methods for config access

func (p *DiscordFOBHABExplosionDamagePlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *DiscordFOBHABExplosionDamagePlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *DiscordFOBHABExplosionDamagePlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}
