package plugin_manager

import (
	"context"
	"fmt"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_loader"
	"go.codycody31.dev/squad-aegis/internal/plugin_sdk"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// LoadedPluginWrapper wraps a loaded .so plugin to implement the Plugin interface
type LoadedPluginWrapper struct {
	loadedPlugin *plugin_loader.LoadedPlugin
	gateway      *plugin_sdk.FeatureGateway
	status       PluginStatus
	config       map[string]interface{}
}

// NewLoadedPluginWrapper creates a new wrapper for a loaded plugin
func NewLoadedPluginWrapper(loadedPlugin *plugin_loader.LoadedPlugin) *LoadedPluginWrapper {
	return &LoadedPluginWrapper{
		loadedPlugin: loadedPlugin,
		status:       PluginStatusStopped,
		config:       make(map[string]interface{}),
	}
}

// GetDefinition returns the plugin definition
func (lpw *LoadedPluginWrapper) GetDefinition() PluginDefinition {
	manifest := lpw.loadedPlugin.Manifest
	
	return PluginDefinition{
		ID:                     manifest.ID,
		Name:                   manifest.Name,
		Description:            manifest.Description,
		Version:                manifest.Version,
		Author:                 manifest.Author,
		AllowMultipleInstances: manifest.AllowMultipleInstances,
		RequiredConnectors:     []string{},
		ConfigSchema:           plug_config_schema.ConfigSchema{Fields: []plug_config_schema.ConfigField{}},
		Events:                 []event_manager.EventType{},
		LongRunning:            manifest.LongRunning,
		CreateInstance:         func() Plugin { return NewLoadedPluginWrapper(lpw.loadedPlugin) },
	}
}

// Initialize sets up the plugin with configuration and dependencies
func (lpw *LoadedPluginWrapper) Initialize(config map[string]interface{}, apis *PluginAPIs) error {
	lpw.config = config
	lpw.status = PluginStatusStarting
	
	// The gateway should be set by the plugin manager before calling Initialize
	// For now, we just initialize the underlying SDK plugin
	if lpw.gateway == nil {
		return fmt.Errorf("feature gateway not set for loaded plugin")
	}
	
	if err := lpw.loadedPlugin.SDK.Initialize(lpw.gateway); err != nil {
		lpw.status = PluginStatusError
		return fmt.Errorf("failed to initialize loaded plugin: %w", err)
	}
	
	lpw.status = PluginStatusRunning
	return nil
}

// SetGateway sets the feature gateway for this plugin
func (lpw *LoadedPluginWrapper) SetGateway(gateway *plugin_sdk.FeatureGateway) {
	lpw.gateway = gateway
}

// Start begins plugin execution (for long-running plugins)
func (lpw *LoadedPluginWrapper) Start(ctx context.Context) error {
	if lpw.status != PluginStatusRunning {
		return fmt.Errorf("plugin must be initialized before starting")
	}
	
	// Loaded plugins manage their own lifecycle through the SDK
	// They can spawn goroutines via the gateway
	return nil
}

// Stop gracefully stops the plugin
func (lpw *LoadedPluginWrapper) Stop() error {
	lpw.status = PluginStatusStopping
	
	if err := lpw.loadedPlugin.SDK.Shutdown(); err != nil {
		lpw.status = PluginStatusError
		return fmt.Errorf("failed to stop loaded plugin: %w", err)
	}
	
	lpw.status = PluginStatusStopped
	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (lpw *LoadedPluginWrapper) HandleEvent(event *PluginEvent) error {
	// Check if plugin has event handling feature
	hasEventHandling := false
	for _, feature := range lpw.loadedPlugin.Manifest.RequiredFeatures {
		if feature == plugin_sdk.FeatureEventHandling {
			hasEventHandling = true
			break
		}
	}
	
	if !hasEventHandling {
		return nil // Plugin doesn't handle events
	}
	
	// Get the event handling API from the gateway
	api, err := lpw.gateway.GetFeatureAPI(plugin_sdk.FeatureEventHandling)
	if err != nil {
		return fmt.Errorf("failed to get event handling API: %w", err)
	}
	
	eventAPI, ok := api.(plugin_sdk.EventHandlingAPI)
	if !ok {
		return fmt.Errorf("event handling API has incorrect type")
	}
	
	// Convert PluginEvent to SDK PluginEvent
	sdkEvent := &plugin_sdk.PluginEvent{
		ID:        event.ID,
		ServerID:  event.ServerID,
		Source:    string(event.Source),
		Type:      event.Type,
		Data:      event.Data,
		Raw:       event.Raw,
		Timestamp: event.Timestamp,
	}
	
	return eventAPI.HandleEvent(sdkEvent)
}

// GetStatus returns the current plugin status
func (lpw *LoadedPluginWrapper) GetStatus() PluginStatus {
	return lpw.status
}

// GetConfig returns the current plugin configuration
func (lpw *LoadedPluginWrapper) GetConfig() map[string]interface{} {
	return lpw.config
}

// UpdateConfig updates the plugin configuration
func (lpw *LoadedPluginWrapper) UpdateConfig(config map[string]interface{}) error {
	lpw.config = config
	// Loaded plugins manage their own config internally
	// They would need to implement a UpdateConfig method in their SDK interface
	// For now, we just store it
	return nil
}

// GetCommands returns the list of commands exposed by this plugin
func (lpw *LoadedPluginWrapper) GetCommands() []PluginCommand {
	// Check if plugin has commands feature
	hasCommands := false
	for _, feature := range lpw.loadedPlugin.Manifest.RequiredFeatures {
		if feature == plugin_sdk.FeatureCommands {
			hasCommands = true
			break
		}
	}
	
	if !hasCommands {
		return []PluginCommand{}
	}
	
	// Get the commands API from the gateway
	api, err := lpw.gateway.GetFeatureAPI(plugin_sdk.FeatureCommands)
	if err != nil {
		return []PluginCommand{}
	}
	
	commandsAPI, ok := api.(plugin_sdk.CommandsAPI)
	if !ok {
		return []PluginCommand{}
	}
	
	sdkCommands := commandsAPI.GetCommands()
	
	// Convert SDK commands to internal commands
	commands := make([]PluginCommand, len(sdkCommands))
	for i, cmd := range sdkCommands {
		commands[i] = PluginCommand{
			ID:                  cmd.ID,
			Name:                cmd.Name,
			Description:         cmd.Description,
			Category:            cmd.Category,
			Parameters:          plug_config_schema.ConfigSchema{Fields: []plug_config_schema.ConfigField{}}, // Simplified
			ExecutionType:       CommandExecutionType(cmd.ExecutionType),
			RequiredPermissions: cmd.RequiredPermissions,
			ConfirmMessage:      cmd.ConfirmMessage,
		}
	}
	
	return commands
}

// ExecuteCommand executes a command with the given parameters
func (lpw *LoadedPluginWrapper) ExecuteCommand(commandID string, params map[string]interface{}) (*CommandResult, error) {
	// Check if plugin has commands feature
	hasCommands := false
	for _, feature := range lpw.loadedPlugin.Manifest.RequiredFeatures {
		if feature == plugin_sdk.FeatureCommands {
			hasCommands = true
			break
		}
	}
	
	if !hasCommands {
		return nil, fmt.Errorf("plugin does not support commands")
	}
	
	// Get the commands API from the gateway
	api, err := lpw.gateway.GetFeatureAPI(plugin_sdk.FeatureCommands)
	if err != nil {
		return nil, fmt.Errorf("failed to get commands API: %w", err)
	}
	
	commandsAPI, ok := api.(plugin_sdk.CommandsAPI)
	if !ok {
		return nil, fmt.Errorf("commands API has incorrect type")
	}
	
	sdkResult, err := commandsAPI.ExecuteCommand(commandID, params)
	if err != nil {
		return nil, err
	}
	
	// Convert SDK result to internal result
	return &CommandResult{
		Success:     sdkResult.Success,
		Message:     sdkResult.Message,
		Data:        sdkResult.Data,
		ExecutionID: sdkResult.ExecutionID,
		Error:       sdkResult.Error,
	}, nil
}

// GetCommandExecutionStatus returns the status of an async command execution
func (lpw *LoadedPluginWrapper) GetCommandExecutionStatus(executionID string) (*CommandExecutionStatus, error) {
	// Check if plugin has commands feature
	hasCommands := false
	for _, feature := range lpw.loadedPlugin.Manifest.RequiredFeatures {
		if feature == plugin_sdk.FeatureCommands {
			hasCommands = true
			break
		}
	}
	
	if !hasCommands {
		return nil, fmt.Errorf("plugin does not support commands")
	}
	
	// Get the commands API from the gateway
	api, err := lpw.gateway.GetFeatureAPI(plugin_sdk.FeatureCommands)
	if err != nil {
		return nil, fmt.Errorf("failed to get commands API: %w", err)
	}
	
	commandsAPI, ok := api.(plugin_sdk.CommandsAPI)
	if !ok {
		return nil, fmt.Errorf("commands API has incorrect type")
	}
	
	sdkStatus, err := commandsAPI.GetCommandExecutionStatus(executionID)
	if err != nil {
		return nil, err
	}
	
	// Convert SDK status to internal status
	var result *CommandResult
	if sdkStatus.Result != nil {
		result = &CommandResult{
			Success:     sdkStatus.Result.Success,
			Message:     sdkStatus.Result.Message,
			Data:        sdkStatus.Result.Data,
			ExecutionID: sdkStatus.Result.ExecutionID,
			Error:       sdkStatus.Result.Error,
		}
	}
	
	return &CommandExecutionStatus{
		ExecutionID: sdkStatus.ExecutionID,
		CommandID:   sdkStatus.CommandID,
		Status:      sdkStatus.Status,
		Progress:    sdkStatus.Progress,
		Message:     sdkStatus.Message,
		Result:      result,
		StartedAt:   sdkStatus.StartedAt,
		CompletedAt: sdkStatus.CompletedAt,
	}, nil
}

