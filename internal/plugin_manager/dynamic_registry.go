package plugin_manager

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_loader"
	"go.codycody31.dev/squad-aegis/internal/plugin_sdk"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// DynamicPluginRegistry extends PluginRegistry to support runtime-loaded plugins
type DynamicPluginRegistry struct {
	builtinRegistry  PluginRegistry
	loadedPlugins    map[string]*plugin_loader.LoadedPlugin // pluginID -> LoadedPlugin
	mu               sync.RWMutex
}

// NewDynamicPluginRegistry creates a new dynamic plugin registry
func NewDynamicPluginRegistry(builtinRegistry PluginRegistry) *DynamicPluginRegistry {
	return &DynamicPluginRegistry{
		builtinRegistry: builtinRegistry,
		loadedPlugins:   make(map[string]*plugin_loader.LoadedPlugin),
	}
}

// RegisterPlugin registers a built-in plugin definition
func (dpr *DynamicPluginRegistry) RegisterPlugin(definition PluginDefinition) error {
	return dpr.builtinRegistry.RegisterPlugin(definition)
}

// RegisterLoadedPlugin registers a runtime-loaded .so plugin
func (dpr *DynamicPluginRegistry) RegisterLoadedPlugin(loadedPlugin *plugin_loader.LoadedPlugin) error {
	dpr.mu.Lock()
	defer dpr.mu.Unlock()
	
	pluginID := loadedPlugin.Manifest.ID
	
	// Check if plugin already registered
	if _, exists := dpr.loadedPlugins[pluginID]; exists {
		return fmt.Errorf("loaded plugin %s is already registered", pluginID)
	}
	
	dpr.loadedPlugins[pluginID] = loadedPlugin
	
	log.Info().
		Str("plugin_id", pluginID).
		Str("plugin_name", loadedPlugin.Manifest.Name).
		Str("version", loadedPlugin.Manifest.Version).
		Msg("Registered loaded plugin")
	
	return nil
}

// UnregisterLoadedPlugin unregisters a runtime-loaded plugin
func (dpr *DynamicPluginRegistry) UnregisterLoadedPlugin(pluginID string) error {
	dpr.mu.Lock()
	defer dpr.mu.Unlock()
	
	if _, exists := dpr.loadedPlugins[pluginID]; !exists {
		return fmt.Errorf("loaded plugin %s is not registered", pluginID)
	}
	
	delete(dpr.loadedPlugins, pluginID)
	
	log.Info().Str("plugin_id", pluginID).Msg("Unregistered loaded plugin")
	
	return nil
}

// GetPlugin returns a plugin definition by ID (checks both builtin and loaded)
func (dpr *DynamicPluginRegistry) GetPlugin(pluginID string) (*PluginDefinition, error) {
	// First check if it's a loaded plugin
	dpr.mu.RLock()
	loadedPlugin, isLoaded := dpr.loadedPlugins[pluginID]
	dpr.mu.RUnlock()
	
	if isLoaded {
		// Convert loaded plugin manifest to PluginDefinition
		return dpr.convertLoadedPluginToDefinition(loadedPlugin), nil
	}
	
	// Fall back to builtin registry
	return dpr.builtinRegistry.GetPlugin(pluginID)
}

// ListPlugins returns all available plugin definitions (builtin + loaded)
func (dpr *DynamicPluginRegistry) ListPlugins() []PluginDefinition {
	// Get builtin plugins
	builtinPlugins := dpr.builtinRegistry.ListPlugins()
	
	// Get loaded plugins
	dpr.mu.RLock()
	loadedDefinitions := make([]PluginDefinition, 0, len(dpr.loadedPlugins))
	for _, loadedPlugin := range dpr.loadedPlugins {
		loadedDefinitions = append(loadedDefinitions, *dpr.convertLoadedPluginToDefinition(loadedPlugin))
	}
	dpr.mu.RUnlock()
	
	// Combine both lists
	allPlugins := make([]PluginDefinition, 0, len(builtinPlugins)+len(loadedDefinitions))
	allPlugins = append(allPlugins, builtinPlugins...)
	allPlugins = append(allPlugins, loadedDefinitions...)
	
	return allPlugins
}

// CreatePluginInstance creates a new plugin instance (supports both builtin and loaded)
func (dpr *DynamicPluginRegistry) CreatePluginInstance(pluginID string) (Plugin, error) {
	// Check if it's a loaded plugin
	dpr.mu.RLock()
	loadedPlugin, isLoaded := dpr.loadedPlugins[pluginID]
	dpr.mu.RUnlock()
	
	if isLoaded {
		// Create a wrapped instance for loaded plugins
		return NewLoadedPluginWrapper(loadedPlugin), nil
	}
	
	// Fall back to builtin registry
	return dpr.builtinRegistry.CreatePluginInstance(pluginID)
}

// GetPluginSource returns whether a plugin is builtin or custom
func (dpr *DynamicPluginRegistry) GetPluginSource(pluginID string) plugin_loader.PluginSource {
	dpr.mu.RLock()
	_, isLoaded := dpr.loadedPlugins[pluginID]
	dpr.mu.RUnlock()
	
	if isLoaded {
		return plugin_loader.PluginSourceCustom
	}
	
	return plugin_loader.PluginSourceBuiltin
}

// GetLoadedPlugin returns a loaded plugin by ID
func (dpr *DynamicPluginRegistry) GetLoadedPlugin(pluginID string) (*plugin_loader.LoadedPlugin, error) {
	dpr.mu.RLock()
	defer dpr.mu.RUnlock()
	
	loadedPlugin, exists := dpr.loadedPlugins[pluginID]
	if !exists {
		return nil, fmt.Errorf("loaded plugin %s not found", pluginID)
	}
	
	return loadedPlugin, nil
}

// convertLoadedPluginToDefinition converts a LoadedPlugin to a PluginDefinition
func (dpr *DynamicPluginRegistry) convertLoadedPluginToDefinition(loadedPlugin *plugin_loader.LoadedPlugin) *PluginDefinition {
	manifest := loadedPlugin.Manifest
	
	// Convert feature IDs to event types (simplified mapping)
	eventTypes := []string{}
	for _, feature := range manifest.RequiredFeatures {
		if feature == plugin_sdk.FeatureEventHandling {
			// Custom plugins will need to implement proper event subscription
			// For now, we don't automatically subscribe to events
			eventTypes = append(eventTypes, "custom_plugin_events")
		}
	}
	
	return &PluginDefinition{
		ID:                     manifest.ID,
		Name:                   manifest.Name,
		Description:            manifest.Description,
		Version:                manifest.Version,
		Author:                 manifest.Author,
		AllowMultipleInstances: manifest.AllowMultipleInstances,
		RequiredConnectors:     []string{}, // Custom plugins don't use old connector system
		ConfigSchema:           plug_config_schema.ConfigSchema{Fields: []plug_config_schema.ConfigField{}}, // Custom plugins manage their own config
		Events:                 []event_manager.EventType{}, // Custom plugins use feature-based event handling
		LongRunning:            manifest.LongRunning,
		CreateInstance: func() Plugin {
			return NewLoadedPluginWrapper(loadedPlugin)
		},
	}
}

// ListLoadedPlugins returns all loaded custom plugins
func (dpr *DynamicPluginRegistry) ListLoadedPlugins() []*plugin_loader.LoadedPlugin {
	dpr.mu.RLock()
	defer dpr.mu.RUnlock()
	
	plugins := make([]*plugin_loader.LoadedPlugin, 0, len(dpr.loadedPlugins))
	for _, p := range dpr.loadedPlugins {
		plugins = append(plugins, p)
	}
	
	return plugins
}

// IsPluginLoaded checks if a plugin is loaded
func (dpr *DynamicPluginRegistry) IsPluginLoaded(pluginID string) bool {
	dpr.mu.RLock()
	defer dpr.mu.RUnlock()
	_, exists := dpr.loadedPlugins[pluginID]
	return exists
}

