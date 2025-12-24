package seeding_mode

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// SeedingModePlugin broadcasts seeding rule messages to players at regular intervals
type SeedingModePlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// State management
	mu               sync.Mutex
	status           plugin_manager.PluginStatus
	ctx              context.Context
	cancel           context.CancelFunc
	broadcastTicker  *time.Ticker
	stopBroadcasting bool
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "seeding_mode",
		Name:                   "Seeding Mode",
		Description:            "The Seeding Mode plugin broadcasts seeding rule messages to players at regular intervals when the server is below a specified player count. It can also be configured to display \"Live\" messages when the server goes live.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{},
		LongRunning:            true,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "interval_ms",
					Description: "Frequency of seeding messages in milliseconds.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     150000, // 2.5 minutes = 2.5 * 60 * 1000
				},
				{
					Name:        "seeding_threshold",
					Description: "Player count required for server not to be in seeding mode.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     50,
				},
				{
					Name:        "seeding_message",
					Description: "Seeding message to display.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "Seeding Rules Active! Fight only over the middle flags! No FOB Hunting!",
				},
				{
					Name:        "live_enabled",
					Description: "Enable \"Live\" messages for when the server goes live.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "live_threshold",
					Description: "Player count required for \"Live\" messages to not be displayed.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     52,
				},
				{
					Name:        "live_message",
					Description: "\"Live\" message to display.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "Live!",
				},
				{
					Name:        "wait_on_new_games",
					Description: "Should the plugin wait to be executed on NEW_GAME event.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "wait_time_on_new_game",
					Description: "The time to wait before checking player counts in seconds.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     30,
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeLogGameEventUnified,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &SeedingModePlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *SeedingModePlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

func (p *SeedingModePlugin) GetCommands() []plugin_manager.PluginCommand {
	return []plugin_manager.PluginCommand{}
}

func (p *SeedingModePlugin) ExecuteCommand(commandID string, params map[string]interface{}) (*plugin_manager.CommandResult, error) {
	return nil, fmt.Errorf("no commands available")
}

func (p *SeedingModePlugin) GetCommandExecutionStatus(executionID string) (*plugin_manager.CommandExecutionStatus, error) {
	return nil, fmt.Errorf("no commands available")
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *SeedingModePlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	p.apis = apis
	p.status = plugin_manager.PluginStatusStopped
	p.stopBroadcasting = false

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
func (p *SeedingModePlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	// Start the broadcast loop
	intervalMS := p.getIntConfig("interval_ms")
	if intervalMS <= 0 {
		intervalMS = 150000 // Default fallback
	}

	p.broadcastTicker = time.NewTicker(time.Duration(intervalMS) * time.Millisecond)

	// Start broadcasting goroutine
	go p.broadcastLoop()

	return nil
}

// Stop gracefully stops the plugin
func (p *SeedingModePlugin) Stop() error {
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
func (p *SeedingModePlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != string(event_manager.EventTypeLogGameEventUnified) {
		return nil // Not interested in this event
	}

	if unifiedEvent, ok := event.Data.(*event_manager.LogGameEventUnifiedData); ok {
		if unifiedEvent.EventType == "NEW_GAME" {
			return p.handleNewGame(event)
		}
	}

	return nil
}

// GetStatus returns the current plugin status
func (p *SeedingModePlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *SeedingModePlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *SeedingModePlugin) UpdateConfig(config map[string]interface{}) error {
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

		// Start new ticker with updated interval
		intervalMS := p.getIntConfig("interval_ms")
		if intervalMS <= 0 {
			intervalMS = 150000
		}
		p.broadcastTicker = time.NewTicker(time.Duration(intervalMS) * time.Millisecond)
	}

	p.apis.LogAPI.Info("Seeding Mode plugin configuration updated", map[string]interface{}{
		"interval_ms":       config["interval_ms"],
		"seeding_threshold": config["seeding_threshold"],
		"wait_on_new_games": config["wait_on_new_games"],
	})

	return nil
}

// handleNewGame processes new game events
func (p *SeedingModePlugin) handleNewGame(rawEvent *plugin_manager.PluginEvent) error {
	if !p.getBoolConfig("wait_on_new_games") {
		return nil // Not configured to wait on new games
	}

	// Temporarily stop broadcasting
	p.mu.Lock()
	p.stopBroadcasting = true
	p.mu.Unlock()

	waitTime := p.getIntConfig("wait_time_on_new_game")
	if waitTime <= 0 {
		waitTime = 30 // Default fallback
	}

	p.apis.LogAPI.Info("New game detected - temporarily stopping broadcasts", map[string]interface{}{
		"wait_time_seconds": waitTime,
	})

	// Wait for the specified time, then resume broadcasting
	go func() {
		timer := time.NewTimer(time.Duration(waitTime) * time.Second)
		defer timer.Stop()

		select {
		case <-timer.C:
			p.mu.Lock()
			p.stopBroadcasting = false
			p.mu.Unlock()
			p.apis.LogAPI.Info("Resuming broadcasts after new game wait period", nil)
		case <-p.ctx.Done():
			return // Plugin is stopping
		}
	}()

	return nil
}

// broadcastLoop handles the periodic broadcasting
func (p *SeedingModePlugin) broadcastLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return // Plugin is stopping
		case <-p.broadcastTicker.C:
			if err := p.broadcast(); err != nil {
				p.apis.LogAPI.Error("Failed to broadcast seeding message", err, nil)
			}
		}
	}
}

// broadcast sends appropriate messages based on player count
func (p *SeedingModePlugin) broadcast() error {
	p.mu.Lock()
	stopBroadcasting := p.stopBroadcasting
	p.mu.Unlock()

	if stopBroadcasting {
		return nil // Currently in wait period after new game
	}

	// Get current player count
	players, err := p.apis.ServerAPI.GetPlayers()
	if err != nil {
		return fmt.Errorf("failed to get players: %w", err)
	}

	// Count online players
	onlinePlayerCount := 0
	for _, player := range players {
		if player.IsOnline {
			onlinePlayerCount++
		}
	}

	if onlinePlayerCount == 0 {
		return nil // No players online, don't broadcast
	}

	seedingThreshold := p.getIntConfig("seeding_threshold")
	liveThreshold := p.getIntConfig("live_threshold")
	liveEnabled := p.getBoolConfig("live_enabled")

	if onlinePlayerCount < seedingThreshold {
		// Server is in seeding mode
		seedingMessage := p.getStringConfig("seeding_message")
		if seedingMessage != "" {
			if err := p.apis.RconAPI.Broadcast(seedingMessage); err != nil {
				return fmt.Errorf("failed to broadcast seeding message: %w", err)
			}
			p.apis.LogAPI.Debug("Broadcasted seeding message", map[string]interface{}{
				"playerCount": onlinePlayerCount,
				"threshold":   seedingThreshold,
				"message":     seedingMessage,
			})
		}
	} else if liveEnabled && onlinePlayerCount < liveThreshold {
		// Server is live but below live threshold
		liveMessage := p.getStringConfig("live_message")
		if liveMessage != "" {
			if err := p.apis.RconAPI.Broadcast(liveMessage); err != nil {
				return fmt.Errorf("failed to broadcast live message: %w", err)
			}
			p.apis.LogAPI.Debug("Broadcasted live message", map[string]interface{}{
				"playerCount": onlinePlayerCount,
				"threshold":   liveThreshold,
				"message":     liveMessage,
			})
		}
	}

	return nil
}

// Helper methods for config access

func (p *SeedingModePlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *SeedingModePlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *SeedingModePlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}
