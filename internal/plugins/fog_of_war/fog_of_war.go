package fog_of_war

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// FogOfWarPlugin can be used to automate setting fog of war mode
type FogOfWarPlugin struct {
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
		ID:                     "fog_of_war",
		Name:                   "Fog of War",
		Description:            "The Fog of War plugin can be used to automate setting fog of war mode.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{},
		LongRunning:            false,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "mode",
					Description: "Fog of war mode to set. 0 = Disabled, 1 = Enabled (default), 2 = Only for enemies.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     1,
				},
				{
					Name:        "delay_ms",
					Description: "Delay before setting fog of war mode in milliseconds.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     10000, // 10 seconds
				},
				{
					Name:        "command_template",
					Description: "The RCON command template to use for setting fog of war. Use {mode} as placeholder.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "AdminSetFogOfWar {mode}",
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeLogNewGame,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &FogOfWarPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *FogOfWarPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *FogOfWarPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
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
func (p *FogOfWarPlugin) Start(ctx context.Context) error {
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
func (p *FogOfWarPlugin) Stop() error {
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

	p.apis.LogAPI.Info("Fog of War plugin stopped", nil)

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *FogOfWarPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != "LOG_NEW_GAME" {
		return nil // Not interested in this event
	}

	return p.handleNewGame(event)
}

// GetStatus returns the current plugin status
func (p *FogOfWarPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *FogOfWarPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *FogOfWarPlugin) UpdateConfig(config map[string]interface{}) error {
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

	return nil
}

// handleNewGame processes new game events
func (p *FogOfWarPlugin) handleNewGame(rawEvent *plugin_manager.PluginEvent) error {
	delayMS := p.getIntConfig("delay_ms")
	if delayMS < 0 {
		delayMS = 10000 // Default fallback
	}

	mode := p.getIntConfig("mode")

	p.apis.LogAPI.Info("New game detected - scheduling fog of war mode change", map[string]interface{}{
		"mode":     mode,
		"delay_ms": delayMS,
	})

	// Set fog of war mode after the specified delay
	go func() {
		timer := time.NewTimer(time.Duration(delayMS) * time.Millisecond)
		defer timer.Stop()

		select {
		case <-timer.C:
			if err := p.setFogOfWar(mode); err != nil {
				p.apis.LogAPI.Error("Failed to set fog of war mode", err, map[string]interface{}{
					"mode": mode,
				})
			} else {
				p.apis.LogAPI.Info("Successfully set fog of war mode", map[string]interface{}{
					"mode": mode,
				})
			}
		case <-p.ctx.Done():
			return // Plugin is stopping
		}
	}()

	return nil
}

// setFogOfWar sets the fog of war mode using RCON
func (p *FogOfWarPlugin) setFogOfWar(mode int) error {
	commandTemplate := p.getStringConfig("command_template")
	if commandTemplate == "" {
		commandTemplate = "AdminSetFogOfWar {mode}" // Default fallback
	}

	// Replace the {mode} placeholder with the actual mode value
	// For now, we'll use a simple approach since we know the command structure
	command := fmt.Sprintf("AdminSetFogOfWar %d", mode)

	// Send the command
	response, err := p.apis.RconAPI.SendCommand(command)
	if err != nil {
		return fmt.Errorf("failed to execute fog of war command '%s': %w", command, err)
	}

	p.apis.LogAPI.Debug("Fog of war command executed", map[string]interface{}{
		"command":  command,
		"response": response,
		"mode":     mode,
	})

	return nil
}

// Helper methods for config access

func (p *FogOfWarPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *FogOfWarPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *FogOfWarPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}
