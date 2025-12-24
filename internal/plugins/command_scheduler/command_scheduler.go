package command_scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// ScheduledCommand represents a command with its scheduling configuration
type ScheduledCommand struct {
	Name       string             `json:"name"`
	Command    string             `json:"command"`
	Enabled    bool               `json:"enabled"`
	Interval   int                `json:"interval"`    // seconds
	OnNewGame  bool               `json:"on_new_game"` // run after new game
	LastRun    time.Time          `json:"last_run"`
	NextRun    time.Time          `json:"next_run"`
	Timer      *time.Timer        `json:"-"`
	CancelFunc context.CancelFunc `json:"-"`
}

// CommandSchedulerPlugin schedules and executes predefined commands at intervals or events
type CommandSchedulerPlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// State management
	mu     sync.Mutex
	status plugin_manager.PluginStatus
	ctx    context.Context
	cancel context.CancelFunc

	// Plugin state
	commands       map[string]*ScheduledCommand
	mainTicker     *time.Ticker
	commandTimers  map[string]*time.Timer
	commandCtxs    map[string]context.Context
	commandCancels map[string]context.CancelFunc
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "command_scheduler",
		Name:                   "Command Scheduler",
		Description:            "The Command Scheduler plugin allows you to schedule predefined commands to run at specific intervals or after certain events like new games.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{},
		LongRunning:            true,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				plug_config_schema.NewArrayObjectField(
					"commands",
					"List of commands to schedule with their configuration",
					true,
					[]plug_config_schema.ConfigField{
						plug_config_schema.NewStringField("name", "Unique name for this command", true, ""),
						plug_config_schema.NewStringField("command", "The RCON command to execute", true, ""),
						plug_config_schema.NewBoolField("enabled", "Whether this command is enabled", false, true),
						plug_config_schema.NewIntField("interval", "Interval in seconds between executions", false, 600),
						plug_config_schema.NewBoolField("on_new_game", "Run this command after new game events", false, false),
					},
					[]interface{}{
						plug_config_schema.CreateDefaultObject([]plug_config_schema.ConfigField{
							plug_config_schema.NewStringField("name", "", true, "AdminReloadServerConfig"),
							plug_config_schema.NewStringField("command", "", true, "AdminReloadServerConfig"),
							plug_config_schema.NewBoolField("enabled", "", false, true),
							plug_config_schema.NewIntField("interval", "", false, 600),
							plug_config_schema.NewBoolField("on_new_game", "", false, true),
						}),
					},
				),
				plug_config_schema.NewIntField(
					"check_interval",
					"How often in seconds to check for commands that need to run",
					false,
					60,
				),
				plug_config_schema.NewIntField(
					"new_game_delay",
					"Delay in seconds after new game before running on_new_game commands",
					false,
					30,
				),
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeLogGameEventUnified,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &CommandSchedulerPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *CommandSchedulerPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

func (p *CommandSchedulerPlugin) GetCommands() []plugin_manager.PluginCommand {
	return []plugin_manager.PluginCommand{}
}

func (p *CommandSchedulerPlugin) ExecuteCommand(commandID string, params map[string]interface{}) (*plugin_manager.CommandResult, error) {
	return nil, fmt.Errorf("no commands available")
}

func (p *CommandSchedulerPlugin) GetCommandExecutionStatus(executionID string) (*plugin_manager.CommandExecutionStatus, error) {
	return nil, fmt.Errorf("no commands available")
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *CommandSchedulerPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	p.apis = apis
	p.status = plugin_manager.PluginStatusStopped
	p.commands = make(map[string]*ScheduledCommand)
	p.commandTimers = make(map[string]*time.Timer)
	p.commandCtxs = make(map[string]context.Context)
	p.commandCancels = make(map[string]context.CancelFunc)

	// Validate config
	definition := p.GetDefinition()
	if err := definition.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Fill defaults
	definition.ConfigSchema.FillDefaults(config)

	// Initialize commands from config
	if err := p.initializeCommands(); err != nil {
		return fmt.Errorf("failed to initialize commands: %w", err)
	}

	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// Start begins plugin execution (for long-running plugins)
func (p *CommandSchedulerPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	// Start main ticker for checking command schedules
	checkInterval := p.getIntConfig("check_interval")
	if checkInterval <= 0 {
		checkInterval = 60
	}
	p.mainTicker = time.NewTicker(time.Duration(checkInterval) * time.Second)

	// Start background goroutine
	go p.schedulerLoop()

	// Schedule initial commands
	p.scheduleAllCommands()

	p.apis.LogAPI.Info("Command Scheduler plugin started", map[string]interface{}{
		"commands_count": len(p.commands),
	})

	return nil
}

// Stop gracefully stops the plugin
func (p *CommandSchedulerPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusStopped {
		return nil // Already stopped
	}

	p.status = plugin_manager.PluginStatusStopping

	// Stop main ticker
	if p.mainTicker != nil {
		p.mainTicker.Stop()
		p.mainTicker = nil
	}

	// Cancel all command timers
	for name, timer := range p.commandTimers {
		if timer != nil {
			timer.Stop()
		}
		delete(p.commandTimers, name)
	}

	// Cancel all command contexts
	for name, cancel := range p.commandCancels {
		if cancel != nil {
			cancel()
		}
		delete(p.commandCancels, name)
		delete(p.commandCtxs, name)
	}

	if p.cancel != nil {
		p.cancel()
	}

	p.status = plugin_manager.PluginStatusStopped

	p.apis.LogAPI.Info("Command Scheduler plugin stopped", nil)

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *CommandSchedulerPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
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
func (p *CommandSchedulerPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *CommandSchedulerPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *CommandSchedulerPlugin) UpdateConfig(config map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Validate new config
	definition := p.GetDefinition()
	if err := definition.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Fill defaults
	definition.ConfigSchema.FillDefaults(config)

	// Stop all current command schedules
	p.stopAllCommandsUnsafe()

	p.config = config

	// Reinitialize commands with new config
	if err := p.initializeCommands(); err != nil {
		return fmt.Errorf("failed to reinitialize commands: %w", err)
	}

	// Reschedule commands if plugin is running
	if p.status == plugin_manager.PluginStatusRunning {
		p.scheduleAllCommands()
	}

	p.apis.LogAPI.Info("Command Scheduler plugin configuration updated", map[string]interface{}{
		"commands_count": len(p.commands),
	})

	return nil
}

// handleNewGame processes new game events
func (p *CommandSchedulerPlugin) handleNewGame(rawEvent *plugin_manager.PluginEvent) error {
	p.apis.LogAPI.Info("New game detected - scheduling on_new_game commands", nil)

	// Get delay from config
	delay := p.getIntConfig("new_game_delay")
	if delay <= 0 {
		delay = 30
	}

	// Schedule commands that should run on new game
	go func() {
		timer := time.NewTimer(time.Duration(delay) * time.Second)
		defer timer.Stop()

		select {
		case <-timer.C:
			p.runOnNewGameCommands()
		case <-p.ctx.Done():
			return // Plugin is stopping
		}
	}()

	return nil
}

// schedulerLoop handles the periodic checking of command schedules
func (p *CommandSchedulerPlugin) schedulerLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return // Plugin is stopping
		case <-p.mainTicker.C:
			p.checkAndRunCommands()
		}
	}
}

// initializeCommands sets up the command structures from config
func (p *CommandSchedulerPlugin) initializeCommands() error {
	p.commands = make(map[string]*ScheduledCommand)

	// Use the schema helper to get array of objects
	commandsObjects := plug_config_schema.GetArrayObjectValue(p.config, "commands")

	for i, cmdObj := range commandsObjects {
		name := plug_config_schema.GetStringValue(cmdObj, "name")
		if name == "" {
			return fmt.Errorf("command at index %d missing or invalid name", i)
		}

		command := plug_config_schema.GetStringValue(cmdObj, "command")
		if command == "" {
			return fmt.Errorf("command '%s' missing or invalid command string", name)
		}

		enabled := plug_config_schema.GetBoolValue(cmdObj, "enabled")
		interval := plug_config_schema.GetIntValue(cmdObj, "interval")
		onNewGame := plug_config_schema.GetBoolValue(cmdObj, "on_new_game")

		// Ensure minimum interval
		if interval <= 0 {
			interval = 600 // default 10 minutes
		}

		scheduledCmd := &ScheduledCommand{
			Name:      name,
			Command:   command,
			Enabled:   enabled,
			Interval:  interval,
			OnNewGame: onNewGame,
			LastRun:   time.Time{},
			NextRun:   time.Now().Add(time.Duration(interval) * time.Second),
		}

		p.commands[name] = scheduledCmd

		p.apis.LogAPI.Debug("Initialized scheduled command", map[string]interface{}{
			"name":        name,
			"command":     command,
			"enabled":     enabled,
			"interval":    interval,
			"on_new_game": onNewGame,
		})
	}

	return nil
}

// scheduleAllCommands sets up timers for all enabled commands
func (p *CommandSchedulerPlugin) scheduleAllCommands() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for name, cmd := range p.commands {
		if cmd.Enabled {
			p.scheduleCommandUnsafe(name, cmd)
		}
	}
}

// scheduleCommandUnsafe schedules a single command (must be called with mutex held)
func (p *CommandSchedulerPlugin) scheduleCommandUnsafe(name string, cmd *ScheduledCommand) {
	// Cancel existing timer if any
	if existingTimer, exists := p.commandTimers[name]; exists {
		existingTimer.Stop()
		delete(p.commandTimers, name)
	}
	if existingCancel, exists := p.commandCancels[name]; exists {
		existingCancel()
		delete(p.commandCancels, name)
		delete(p.commandCtxs, name)
	}

	if !cmd.Enabled || cmd.Interval <= 0 {
		return
	}

	// Calculate next run time
	now := time.Now()
	nextRun := cmd.NextRun
	if nextRun.IsZero() || nextRun.Before(now) {
		nextRun = now.Add(time.Duration(cmd.Interval) * time.Second)
	}

	// Create context for this command
	cmdCtx, cmdCancel := context.WithCancel(p.ctx)
	p.commandCtxs[name] = cmdCtx
	p.commandCancels[name] = cmdCancel

	// Create timer
	duration := nextRun.Sub(now)
	timer := time.NewTimer(duration)
	p.commandTimers[name] = timer

	// Update next run time
	cmd.NextRun = nextRun

	p.apis.LogAPI.Debug("Scheduled command", map[string]interface{}{
		"name":     name,
		"command":  cmd.Command,
		"duration": duration.String(),
		"next_run": nextRun.Format(time.RFC3339),
	})

	// Start goroutine to wait for timer
	go func() {
		select {
		case <-timer.C:
			p.runCommand(name)
		case <-cmdCtx.Done():
			return // Command was cancelled
		}
	}()
}

// runCommand executes a scheduled command
func (p *CommandSchedulerPlugin) runCommand(name string) {
	p.mu.Lock()
	cmd, exists := p.commands[name]
	if !exists || !cmd.Enabled {
		p.mu.Unlock()
		return
	}

	p.apis.LogAPI.Info("Running scheduled command", map[string]interface{}{
		"name":    name,
		"command": cmd.Command,
	})

	// Update last run time
	cmd.LastRun = time.Now()

	// Calculate next run time
	cmd.NextRun = cmd.LastRun.Add(time.Duration(cmd.Interval) * time.Second)

	// Clean up old timer
	if _, exists := p.commandTimers[name]; exists {
		delete(p.commandTimers, name)
	}
	if cancel, exists := p.commandCancels[name]; exists {
		cancel()
		delete(p.commandCancels, name)
		delete(p.commandCtxs, name)
	}

	p.mu.Unlock()

	// Execute the command via RCON
	if _, err := p.apis.RconAPI.SendCommand(cmd.Command); err != nil {
		p.apis.LogAPI.Error("Failed to execute scheduled command", err, map[string]interface{}{
			"name":    name,
			"command": cmd.Command,
		})
	} else {
		p.apis.LogAPI.Info("Successfully executed scheduled command", map[string]interface{}{
			"name":    name,
			"command": cmd.Command,
		})
	}

	// Reschedule the command
	p.mu.Lock()
	if cmd.Enabled { // Check if still enabled (might have been disabled)
		p.scheduleCommandUnsafe(name, cmd)
	}
	p.mu.Unlock()
}

// runOnNewGameCommands executes commands that should run after a new game
func (p *CommandSchedulerPlugin) runOnNewGameCommands() {
	p.mu.Lock()
	commandsToRun := make([]string, 0)

	for name, cmd := range p.commands {
		if cmd.Enabled && cmd.OnNewGame {
			commandsToRun = append(commandsToRun, name)
		}
	}
	p.mu.Unlock()

	p.apis.LogAPI.Info("Running on_new_game commands", map[string]interface{}{
		"commands": commandsToRun,
	})

	for _, name := range commandsToRun {
		p.runCommand(name)
	}
}

// checkAndRunCommands checks if any commands need to run and executes them
func (p *CommandSchedulerPlugin) checkAndRunCommands() {
	p.mu.Lock()
	now := time.Now()
	commandsToRun := make([]string, 0)

	for name, cmd := range p.commands {
		if cmd.Enabled && cmd.Interval > 0 && !cmd.NextRun.IsZero() && now.After(cmd.NextRun) {
			commandsToRun = append(commandsToRun, name)
		}
	}
	p.mu.Unlock()

	for _, name := range commandsToRun {
		p.runCommand(name)
	}
}

// stopAllCommandsUnsafe stops all command timers (must be called with mutex held)
func (p *CommandSchedulerPlugin) stopAllCommandsUnsafe() {
	for name := range p.commands {
		if timer, exists := p.commandTimers[name]; exists {
			timer.Stop()
			delete(p.commandTimers, name)
		}
		if cancel, exists := p.commandCancels[name]; exists {
			cancel()
			delete(p.commandCancels, name)
			delete(p.commandCtxs, name)
		}
	}
}

// Helper methods for config access

func (p *CommandSchedulerPlugin) getStringConfig(key string) string {
	return plug_config_schema.GetStringValue(p.config, key)
}

func (p *CommandSchedulerPlugin) getIntConfig(key string) int {
	return plug_config_schema.GetIntValue(p.config, key)
}

func (p *CommandSchedulerPlugin) getBoolConfig(key string) bool {
	return plug_config_schema.GetBoolValue(p.config, key)
}
