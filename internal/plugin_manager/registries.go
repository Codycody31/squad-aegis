package plugin_manager

import (
	"fmt"
	"sync"
)

// pluginRegistry implements PluginRegistry interface
type pluginRegistry struct {
	plugins map[string]PluginDefinition
	mu      sync.RWMutex
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry() PluginRegistry {
	return &pluginRegistry{
		plugins: make(map[string]PluginDefinition),
	}
}

func (r *pluginRegistry) RegisterPlugin(definition PluginDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if definition.ID == "" {
		return fmt.Errorf("plugin ID cannot be empty")
	}

	if definition.CreateInstance == nil {
		return fmt.Errorf("plugin %s must have a CreateInstance function", definition.ID)
	}

	if _, exists := r.plugins[definition.ID]; exists {
		return fmt.Errorf("plugin %s is already registered", definition.ID)
	}

	r.plugins[definition.ID] = definition
	return nil
}

func (r *pluginRegistry) GetPlugin(pluginID string) (*PluginDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	definition, exists := r.plugins[pluginID]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", pluginID)
	}

	return &definition, nil
}

func (r *pluginRegistry) ListPlugins() []PluginDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]PluginDefinition, 0, len(r.plugins))
	for _, definition := range r.plugins {
		plugins = append(plugins, definition)
	}

	return plugins
}

func (r *pluginRegistry) CreatePluginInstance(pluginID string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	definition, exists := r.plugins[pluginID]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", pluginID)
	}

	return definition.CreateInstance(), nil
}

// connectorRegistry implements ConnectorRegistry interface
type connectorRegistry struct {
	connectors map[string]ConnectorDefinition
	mu         sync.RWMutex
}

// NewConnectorRegistry creates a new connector registry
func NewConnectorRegistry() ConnectorRegistry {
	return &connectorRegistry{
		connectors: make(map[string]ConnectorDefinition),
	}
}

func (r *connectorRegistry) RegisterConnector(definition ConnectorDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if definition.ID == "" {
		return fmt.Errorf("connector ID cannot be empty")
	}

	if definition.CreateInstance == nil {
		return fmt.Errorf("connector %s must have a CreateInstance function", definition.ID)
	}

	if _, exists := r.connectors[definition.ID]; exists {
		return fmt.Errorf("connector %s is already registered", definition.ID)
	}

	r.connectors[definition.ID] = definition
	return nil
}

func (r *connectorRegistry) GetConnector(connectorID string) (*ConnectorDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	definition, exists := r.connectors[connectorID]
	if !exists {
		return nil, fmt.Errorf("connector %s not found", connectorID)
	}

	return &definition, nil
}

func (r *connectorRegistry) ListConnectors() []ConnectorDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	connectors := make([]ConnectorDefinition, 0, len(r.connectors))
	for _, definition := range r.connectors {
		connectors = append(connectors, definition)
	}

	return connectors
}

func (r *connectorRegistry) CreateConnectorInstance(connectorID string) (Connector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	definition, exists := r.connectors[connectorID]
	if !exists {
		return nil, fmt.Errorf("connector %s not found", connectorID)
	}

	return definition.CreateInstance(), nil
}
