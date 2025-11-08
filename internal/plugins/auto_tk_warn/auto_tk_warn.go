package auto_tk_warn

import (
	"context"
	"fmt"
	"sync"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// AutoTKWarnPlugin automatically warns players when they teamkill
type AutoTKWarnPlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// State management
	mu     sync.Mutex
	status plugin_manager.PluginStatus
	ctx    context.Context
	cancel context.CancelFunc
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "auto_tk_warn",
		Name:                   "Auto TK Warn",
		Description:            "The Auto TK Warn plugin will automatically warn players with a message when they teamkill.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{},
		LongRunning:            false,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "attacker_message",
					Description: "The message to warn attacking players with.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "Please apologise for ALL TKs in ALL chat!",
				},
				{
					Name:        "victim_message",
					Description: "The message that will be sent to the victim.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "You have been TK'd...",
				},
				{
					Name:        "warn_attacker",
					Description: "Whether to warn the attacker.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "warn_victim",
					Description: "Whether to warn the victim.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     false,
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeLogPlayerDied,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &AutoTKWarnPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *AutoTKWarnPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *AutoTKWarnPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
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

	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// Start begins plugin execution (for long-running plugins)
func (p *AutoTKWarnPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	return nil
}

// Stop gracefully stops the plugin
func (p *AutoTKWarnPlugin) Stop() error {
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
func (p *AutoTKWarnPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Data.(*event_manager.LogPlayerDiedData).Teamkill {
		return p.handleTeamkill(event)
	}

	return nil
}

// GetStatus returns the current plugin status
func (p *AutoTKWarnPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *AutoTKWarnPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *AutoTKWarnPlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Auto TK Warn plugin configuration updated", map[string]interface{}{
		"warn_attacker": config["warn_attacker"],
		"warn_victim":   config["warn_victim"],
	})

	return nil
}

// handleTeamkill processes teamkill events
func (p *AutoTKWarnPlugin) handleTeamkill(rawEvent *plugin_manager.PluginEvent) error {
	event, ok := rawEvent.Data.(*event_manager.LogPlayerDiedData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	if event.AttackerEOS == "" || event.VictimName == "" || !event.Teamkill {
		return nil // Not a teamkill or missing data
	}

	warnAttacker := p.getBoolConfig("warn_attacker")
	warnVictim := p.getBoolConfig("warn_victim")
	attackerMessage := p.getStringConfig("attacker_message")
	victimMessage := p.getStringConfig("victim_message")

	// Warn the attacker if configured
	if warnAttacker && attackerMessage != "" && event.AttackerSteam != "" {
		if err := p.apis.RconAPI.SendWarningToPlayer(event.AttackerSteam, attackerMessage); err != nil {
			p.apis.LogAPI.Error("Failed to warn teamkill attacker", err, map[string]interface{}{
				"attacker_steam_id": event.AttackerSteam,
				"attacker_eos_id":   event.AttackerEOS,
				"victim_name":       event.VictimName,
			})
		} else {
			p.apis.LogAPI.Info("Warned teamkill attacker", map[string]interface{}{
				"attacker_steam_id": event.AttackerSteam,
				"attacker_eos_id":   event.AttackerEOS,
				"victim_name":       event.VictimName,
				"weapon":            event.Weapon,
			})
		}
	}

	// Warn the victim if configured
	if warnVictim && victimMessage != "" {
		// For victims, we need to try to find their Steam ID from the player list
		// since the teamkill event might not have the victim's Steam ID directly
		victimSteamID, err := p.findPlayerSteamID(event.VictimName)
		if err != nil {
			p.apis.LogAPI.Debug("Could not find victim Steam ID for warning", map[string]interface{}{
				"victim_name": event.VictimName,
				"error":       err.Error(),
			})
			return nil // Don't fail the entire event processing
		}

		if victimSteamID != "" {
			if err := p.apis.RconAPI.SendWarningToPlayer(victimSteamID, victimMessage); err != nil {
				p.apis.LogAPI.Error("Failed to warn teamkill victim", err, map[string]interface{}{
					"victim_name":     event.VictimName,
					"victim_steam_id": victimSteamID,
					"attacker_eos":    event.AttackerEOS,
				})
			} else {
				p.apis.LogAPI.Info("Warned teamkill victim", map[string]interface{}{
					"victim_name":     event.VictimName,
					"victim_steam_id": victimSteamID,
					"attacker_eos":    event.AttackerEOS,
					"weapon":          event.Weapon,
				})
			}
		}
	}

	return nil
}

// findPlayerSteamID attempts to find a player's Steam ID by their name
func (p *AutoTKWarnPlugin) findPlayerSteamID(playerName string) (string, error) {
	players, err := p.apis.ServerAPI.GetPlayers()
	if err != nil {
		return "", fmt.Errorf("failed to get player list: %w", err)
	}

	// Look for player by exact name match
	for _, player := range players {
		if player.Name == playerName && player.IsOnline {
			return player.SteamID, nil
		}
	}

	return "", fmt.Errorf("player not found: %s", playerName)
}

// Helper methods for config access

func (p *AutoTKWarnPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *AutoTKWarnPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}
