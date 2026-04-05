package plugin_manager

import (
	"fmt"
	"strings"
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

	if definition.Source == "" {
		definition.Source = PluginSourceBundled
	}
	if definition.Source == PluginSourceBundled {
		if definition.Distribution == "" {
			definition.Distribution = PluginDistributionBundled
		}
		definition.Official = true
		if definition.InstallState == "" {
			definition.InstallState = PluginInstallStateReady
		}
	}
	if definition.Source == PluginSourceNative && definition.InstallState == "" {
		definition.InstallState = PluginInstallStateReady
	}

	existing, exists := r.plugins[definition.ID]
	if exists && !(existing.Source == PluginSourceNative && definition.Source == PluginSourceNative) {
		return fmt.Errorf("plugin %s is already registered", definition.ID)
	}

	r.plugins[definition.ID] = definition
	return nil
}

func (r *pluginRegistry) UnregisterPlugin(pluginID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.plugins, pluginID)
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
	connectors map[string]ConnectorDefinition // canonical connector ID -> definition
	aliases    map[string]string              // legacy/instance key/alias -> canonical ID
	mu         sync.RWMutex
}

// NewConnectorRegistry creates a new connector registry
func NewConnectorRegistry() ConnectorRegistry {
	return &connectorRegistry{
		connectors: make(map[string]ConnectorDefinition),
		aliases:    make(map[string]string),
	}
}

func (r *connectorRegistry) removeConnectorAliasesLocked(canonicalID string) {
	for alias, target := range r.aliases {
		if target == canonicalID || alias == canonicalID {
			delete(r.aliases, alias)
		}
	}
}

func (r *connectorRegistry) registerConnectorAliasesLocked(canonicalID string, definition ConnectorDefinition) {
	r.aliases[canonicalID] = canonicalID
	storageKey := definition.ConnectorInstanceStorageKey()
	if storageKey != "" && storageKey != canonicalID {
		r.aliases[storageKey] = canonicalID
	}
	for _, legacy := range definition.LegacyIDs {
		legacy = strings.TrimSpace(legacy)
		if legacy == "" {
			continue
		}
		if other, exists := r.aliases[legacy]; exists && other != canonicalID {
			// Should not happen if callers unregister native replacements first.
			delete(r.aliases, legacy)
		}
		r.aliases[legacy] = canonicalID
	}
}

// resolveCanonicalConnectorIDRefLocked maps a legacy or canonical reference to a canonical ID (call with r.mu RLock held).
func (r *connectorRegistry) resolveCanonicalConnectorIDRefLocked(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("connector ID cannot be empty")
	}
	if canonical, ok := r.aliases[ref]; ok {
		return canonical, nil
	}
	if _, ok := r.connectors[ref]; ok {
		return ref, nil
	}
	return "", fmt.Errorf("connector %s not found", ref)
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

	canonical := strings.TrimSpace(definition.ID)

	if existing, exists := r.connectors[canonical]; exists {
		sameNative := existing.Source == PluginSourceNative && definition.Source == PluginSourceNative
		if !sameNative {
			return fmt.Errorf("connector %s is already registered", canonical)
		}
		r.removeConnectorAliasesLocked(canonical)
	}

	if definition.Source == "" {
		definition.Source = PluginSourceBundled
	}
	if definition.Source == PluginSourceBundled {
		if definition.Distribution == "" {
			definition.Distribution = PluginDistributionBundled
		}
		definition.Official = true
		if definition.InstallState == "" {
			definition.InstallState = PluginInstallStateReady
		}
	}
	if definition.Source == PluginSourceNative && definition.InstallState == "" {
		definition.InstallState = PluginInstallStateReady
	}

	r.connectors[canonical] = definition
	r.registerConnectorAliasesLocked(canonical, definition)
	return nil
}

func (r *connectorRegistry) UnregisterConnector(canonicalID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	canonicalID = strings.TrimSpace(canonicalID)
	delete(r.connectors, canonicalID)
	r.removeConnectorAliasesLocked(canonicalID)
}

func (r *connectorRegistry) GetConnector(connectorID string) (*ConnectorDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	canonical, err := r.resolveCanonicalConnectorIDRefLocked(connectorID)
	if err != nil {
		return nil, err
	}

	definition, exists := r.connectors[canonical]
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

	canonical, err := r.resolveCanonicalConnectorIDRefLocked(connectorID)
	if err != nil {
		return nil, err
	}

	definition, exists := r.connectors[canonical]
	if !exists {
		return nil, fmt.Errorf("connector %s not found", connectorID)
	}

	return definition.CreateInstance(), nil
}
