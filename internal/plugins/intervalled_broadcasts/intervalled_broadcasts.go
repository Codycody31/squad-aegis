package intervalled_broadcasts

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// IntervalledBroadcastsPlugin allows you to set broadcasts which will be broadcasted at preset intervals
type IntervalledBroadcastsPlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// State management
	mu              sync.Mutex
	status          plugin_manager.PluginStatus
	ctx             context.Context
	cancel          context.CancelFunc
	broadcastTicker *time.Ticker
	currentIndex    int
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "intervalled_broadcasts",
		Name:                   "Intervalled Broadcasts",
		Description:            "The Intervalled Broadcasts plugin allows you to set broadcasts, which will be broadcasted at preset intervals.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{},
		LongRunning:            true,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "broadcasts",
					Description: "Messages to broadcast.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeArrayString,
					Default:     []interface{}{"This server is powered by Squad Aegis."},
				},
				{
					Name:        "interval_ms",
					Description: "Frequency of the broadcasts in milliseconds.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     300000, // 5 minutes = 5 * 60 * 1000
				},
				{
					Name:        "enabled",
					Description: "Whether the plugin is enabled.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "shuffle_messages",
					Description: "Whether to shuffle through messages in order or randomly.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     false,
				},
			},
		},

		Events: []event_manager.EventType{},

		CreateInstance: func() plugin_manager.Plugin {
			return &IntervalledBroadcastsPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *IntervalledBroadcastsPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *IntervalledBroadcastsPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	p.apis = apis
	p.status = plugin_manager.PluginStatusStopped
	p.currentIndex = 0

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
func (p *IntervalledBroadcastsPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	// Check if plugin is enabled
	if !p.getBoolConfig("enabled") {
		p.apis.LogAPI.Info("Intervalled Broadcasts plugin is disabled", nil)
		return nil
	}

	// Check if we have any broadcasts configured
	broadcasts := p.getStringArrayConfig("broadcasts")
	if len(broadcasts) == 0 {
		p.apis.LogAPI.Warn("No broadcasts configured for Intervalled Broadcasts plugin", nil)
		return nil
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	// Start the broadcast loop
	intervalMS := p.getIntConfig("interval_ms")
	if intervalMS <= 0 {
		intervalMS = 300000 // Default fallback
	}

	p.broadcastTicker = time.NewTicker(time.Duration(intervalMS) * time.Millisecond)

	// Start broadcasting goroutine
	go p.broadcastLoop()

	return nil
}

// Stop gracefully stops the plugin
func (p *IntervalledBroadcastsPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusStopped {
		return nil // Already stopped
	}

	p.status = plugin_manager.PluginStatusStopping

	if p.broadcastTicker != nil {
		p.broadcastTicker.Stop()
		p.broadcastTicker = nil
	}

	if p.cancel != nil {
		p.cancel()
	}

	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *IntervalledBroadcastsPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	// This plugin doesn't handle any events
	return nil
}

// GetStatus returns the current plugin status
func (p *IntervalledBroadcastsPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *IntervalledBroadcastsPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *IntervalledBroadcastsPlugin) UpdateConfig(config map[string]interface{}) error {
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

	// If running, restart with new config
	if p.status == plugin_manager.PluginStatusRunning {
		// Stop current ticker
		if p.broadcastTicker != nil {
			p.broadcastTicker.Stop()
		}

		// Check if still enabled
		if !p.getBoolConfig("enabled") {
			p.apis.LogAPI.Info("Intervalled Broadcasts plugin disabled via config update", nil)
			return nil
		}

		// Start new ticker with updated interval
		intervalMS := p.getIntConfig("interval_ms")
		if intervalMS <= 0 {
			intervalMS = 300000
		}
		p.broadcastTicker = time.NewTicker(time.Duration(intervalMS) * time.Millisecond)
	}

	p.apis.LogAPI.Info("Intervalled Broadcasts plugin configuration updated", map[string]interface{}{
		"interval_ms":     config["interval_ms"],
		"broadcast_count": len(p.getStringArrayConfig("broadcasts")),
		"enabled":         config["enabled"],
	})

	return nil
}

// broadcastLoop handles the periodic broadcasting
func (p *IntervalledBroadcastsPlugin) broadcastLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return // Plugin is stopping
		case <-p.broadcastTicker.C:
			if err := p.broadcast(); err != nil {
				p.apis.LogAPI.Error("Failed to broadcast message", err, nil)
			}
		}
	}
}

// broadcast sends the next message in the rotation
func (p *IntervalledBroadcastsPlugin) broadcast() error {
	broadcasts := p.getStringArrayConfig("broadcasts")
	if len(broadcasts) == 0 {
		return nil // No messages to broadcast
	}

	p.mu.Lock()

	// Get the current message
	currentMessage := broadcasts[p.currentIndex]

	// Move to next message (rotate through the array)
	p.currentIndex = (p.currentIndex + 1) % len(broadcasts)

	p.mu.Unlock()

	// Send the broadcast
	if err := p.apis.RconAPI.Broadcast(currentMessage); err != nil {
		return fmt.Errorf("failed to broadcast message: %w", err)
	}

	p.apis.LogAPI.Debug("Broadcasted message", map[string]interface{}{
		"message": currentMessage,
		"index":   (p.currentIndex - 1 + len(broadcasts)) % len(broadcasts), // Previous index
	})

	return nil
}

// Helper methods for config access

func (p *IntervalledBroadcastsPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *IntervalledBroadcastsPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}

func (p *IntervalledBroadcastsPlugin) getStringArrayConfig(key string) []string {
	if value, ok := p.config[key].([]interface{}); ok {
		result := make([]string, len(value))
		for i, v := range value {
			if str, ok := v.(string); ok {
				result[i] = str
			}
		}
		return result
	}
	return []string{}
}
