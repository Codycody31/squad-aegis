package main

import (
	"fmt"
	"sync"

	"go.codycody31.dev/squad-aegis/internal/plugin_sdk"
)

// ExamplePlugin is a simple example plugin demonstrating the SDK
type ExamplePlugin struct {
	baseAPI plugin_sdk.BaseAPI
	mu      sync.Mutex
	config  map[string]interface{}
}

// PluginExport is the exported symbol that the plugin loader will look for
var PluginExport plugin_sdk.PluginSDK = &ExamplePlugin{}

// GetManifest returns the plugin's manifest
func (p *ExamplePlugin) GetManifest() plugin_sdk.PluginManifest {
	return plugin_sdk.PluginManifest{
		ID:          "example_custom_plugin",
		Name:        "Example Custom Plugin",
		Version:     "1.0.0",
		Author:      "Squad Aegis Team",
		SDKVersion:  plugin_sdk.APIVersion,
		Description: "A simple example plugin demonstrating the custom plugin system",
		Website:     "https://github.com/codycody31/squad-aegis",
		
		RequiredFeatures: []plugin_sdk.FeatureID{
			plugin_sdk.FeatureLogging,
			plugin_sdk.FeatureEventHandling,
			plugin_sdk.FeatureServerAPI,
		},
		
		ProvidedFeatures: []plugin_sdk.FeatureID{},
		
		RequiredPermissions: []plugin_sdk.PermissionID{
			plugin_sdk.PermissionEventPublish,
		},
		
		AllowMultipleInstances: false,
		LongRunning:            true,
	}
}

// Initialize is called when the plugin is loaded
func (p *ExamplePlugin) Initialize(baseAPI plugin_sdk.BaseAPI) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.baseAPI = baseAPI
	p.config = make(map[string]interface{})
	
	// Get the logging API
	logAPI, err := baseAPI.GetFeatureAPI(plugin_sdk.FeatureLogging)
	if err != nil {
		return fmt.Errorf("failed to get logging API: %w", err)
	}
	
	log, ok := logAPI.(plugin_sdk.LogAPI)
	if !ok {
		return fmt.Errorf("logging API has incorrect type")
	}
	
	log.Info("Example plugin initialized!", map[string]interface{}{
		"plugin_id":         p.GetManifest().ID,
		"server_id":         baseAPI.GetServerID().String(),
		"plugin_instance_id": baseAPI.GetPluginInstanceID().String(),
	})
	
	// Start a background goroutine to demonstrate tracking
	p.baseAPI.SpawnGoroutine(func() {
		p.backgroundTask()
	})
	
	return nil
}

// backgroundTask demonstrates a long-running goroutine
func (p *ExamplePlugin) backgroundTask() {
	ctx := p.baseAPI.GetContext()
	
	// Get the logging API
	logAPI, err := p.baseAPI.GetFeatureAPI(plugin_sdk.FeatureLogging)
	if err != nil {
		return
	}
	
	log, ok := logAPI.(plugin_sdk.LogAPI)
	if !ok {
		return
	}
	
	log.Info("Background task started", nil)
	
	// Wait for shutdown
	<-ctx.Done()
	
	log.Info("Background task stopped", nil)
}

// Shutdown is called when the plugin is being unloaded
func (p *ExamplePlugin) Shutdown() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Get the logging API
	logAPI, err := p.baseAPI.GetFeatureAPI(plugin_sdk.FeatureLogging)
	if err != nil {
		return fmt.Errorf("failed to get logging API: %w", err)
	}
	
	log, ok := logAPI.(plugin_sdk.LogAPI)
	if !ok {
		return fmt.Errorf("logging API has incorrect type")
	}
	
	log.Info("Example plugin shutting down", nil)
	
	return nil
}

// HandleEvent processes events (implements EventHandlingAPI)
func (p *ExamplePlugin) HandleEvent(event *plugin_sdk.PluginEvent) error {
	// Get the logging API
	logAPI, err := p.baseAPI.GetFeatureAPI(plugin_sdk.FeatureLogging)
	if err != nil {
		return fmt.Errorf("failed to get logging API: %w", err)
	}
	
	log, ok := logAPI.(plugin_sdk.LogAPI)
	if !ok {
		return fmt.Errorf("logging API has incorrect type")
	}
	
	log.Debug("Received event", map[string]interface{}{
		"event_type": event.Type,
		"event_id":   event.ID.String(),
	})
	
	return nil
}

