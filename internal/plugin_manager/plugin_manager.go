package plugin_manager

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
)

// PluginManager manages plugin instances for servers
type PluginManager struct {
	// Plugin management
	plugins  map[uuid.UUID]map[uuid.UUID]*PluginInstance // serverID -> instanceID -> instance
	registry PluginRegistry
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc

	// Dependencies
	db           *sql.DB
	eventManager *event_manager.EventManager
	rconManager  *rcon_manager.RconManager

	// Connector management
	connectors        map[string]*ConnectorInstance
	connectorRegistry ConnectorRegistry
	connectorMu       sync.RWMutex

	// Event subscription
	eventSubscriber *event_manager.EventSubscriber
}

// NewPluginManager creates a new plugin manager
func NewPluginManager(ctx context.Context, db *sql.DB, eventManager *event_manager.EventManager, rconManager *rcon_manager.RconManager) *PluginManager {
	ctx, cancel := context.WithCancel(ctx)

	pm := &PluginManager{
		plugins:           make(map[uuid.UUID]map[uuid.UUID]*PluginInstance),
		registry:          NewPluginRegistry(),
		connectors:        make(map[string]*ConnectorInstance),
		connectorRegistry: NewConnectorRegistry(),
		db:                db,
		eventManager:      eventManager,
		rconManager:       rconManager,
		ctx:               ctx,
		cancel:            cancel,
	}

	// Subscribe to events for plugin distribution
	pm.eventSubscriber = pm.eventManager.Subscribe(event_manager.EventFilter{}, nil, 1000)

	return pm
}

// Start starts the plugin manager
func (pm *PluginManager) Start() error {
	log.Info().Msg("Starting plugin manager")

	// Start connectors first
	if err := pm.startConnectors(); err != nil {
		return fmt.Errorf("failed to start connectors: %w", err)
	}

	// Load and start plugins
	if err := pm.loadPluginsFromDatabase(); err != nil {
		return fmt.Errorf("failed to load plugins from database: %w", err)
	}

	// Start event distribution goroutine
	go pm.eventDistributionLoop()

	log.Info().Msg("Plugin manager started successfully")
	return nil
}

// Stop stops the plugin manager
func (pm *PluginManager) Stop() error {
	log.Info().Msg("Stopping plugin manager")

	pm.cancel()

	// Stop all plugins
	pm.mu.Lock()
	for serverID, serverPlugins := range pm.plugins {
		for instanceID, instance := range serverPlugins {
			if err := pm.stopPluginInstance(instance); err != nil {
				log.Error().
					Str("serverID", serverID.String()).
					Str("instanceID", instanceID.String()).
					Str("pluginID", instance.PluginID).
					Err(err).
					Msg("Failed to stop plugin instance")
			}
		}
	}
	pm.mu.Unlock()

	// Stop all connectors
	pm.connectorMu.Lock()
	for connectorID, instance := range pm.connectors {
		if err := pm.stopConnectorInstance(instance); err != nil {
			log.Error().
				Str("connectorID", connectorID).
				Err(err).
				Msg("Failed to stop connector instance")
		}
	}
	pm.connectorMu.Unlock()

	// Unsubscribe from events
	if pm.eventSubscriber != nil {
		pm.eventManager.Unsubscribe(pm.eventSubscriber.ID)
	}

	log.Info().Msg("Plugin manager stopped")
	return nil
}

// RegisterPlugin registers a new plugin definition
func (pm *PluginManager) RegisterPlugin(definition PluginDefinition) error {
	return pm.registry.RegisterPlugin(definition)
}

// RegisterConnector registers a new connector definition
func (pm *PluginManager) RegisterConnector(definition ConnectorDefinition) error {
	return pm.connectorRegistry.RegisterConnector(definition)
}

// CreatePluginInstance creates and starts a new plugin instance
func (pm *PluginManager) CreatePluginInstance(serverID uuid.UUID, pluginID string, name string, config map[string]interface{}) (*PluginInstance, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Get plugin definition
	definition, err := pm.registry.GetPlugin(pluginID)
	if err != nil {
		return nil, fmt.Errorf("plugin not found: %w", err)
	}

	// Check if multiple instances are allowed
	if !definition.AllowMultipleInstances {
		if serverPlugins, exists := pm.plugins[serverID]; exists {
			for _, instance := range serverPlugins {
				if instance.PluginID == pluginID {
					return nil, fmt.Errorf("plugin %s does not allow multiple instances on server %s", pluginID, serverID.String())
				}
			}
		}
	}

	// Validate required connectors
	for _, connectorID := range definition.RequiredConnectors {
		if _, exists := pm.connectors[connectorID]; !exists {
			return nil, fmt.Errorf("required connector %s is not available", connectorID)
		}
		if pm.connectors[connectorID].Status != ConnectorStatusRunning {
			return nil, fmt.Errorf("required connector %s is not running", connectorID)
		}
	}

	// Create plugin instance
	plugin, err := pm.registry.CreatePluginInstance(pluginID)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin instance: %w", err)
	}

	// Create instance record
	instanceID := uuid.New()
	ctx, cancel := context.WithCancel(pm.ctx)

	instance := &PluginInstance{
		ID:        instanceID,
		ServerID:  serverID,
		PluginID:  pluginID,
		Name:      name,
		Config:    config,
		Status:    PluginStatusStopped,
		Enabled:   true,
		Plugin:    plugin,
		Context:   ctx,
		Cancel:    cancel,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Initialize server plugins map if needed
	if pm.plugins[serverID] == nil {
		pm.plugins[serverID] = make(map[uuid.UUID]*PluginInstance)
	}

	// Store instance
	pm.plugins[serverID][instanceID] = instance

	// Save to database
	if err := pm.savePluginInstanceToDatabase(instance); err != nil {
		delete(pm.plugins[serverID], instanceID)
		cancel()
		return nil, fmt.Errorf("failed to save plugin instance to database: %w", err)
	}

	// Initialize and start plugin
	if err := pm.initializePluginInstance(instance); err != nil {
		delete(pm.plugins[serverID], instanceID)
		cancel()
		return nil, fmt.Errorf("failed to initialize plugin instance: %w", err)
	}

	log.Info().
		Str("serverID", serverID.String()).
		Str("instanceID", instanceID.String()).
		Str("pluginID", pluginID).
		Str("name", name).
		Msg("Created plugin instance")

	return instance, nil
}

// GetPluginInstances returns all plugin instances for a server
func (pm *PluginManager) GetPluginInstances(serverID uuid.UUID) []*PluginInstance {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	serverPlugins, exists := pm.plugins[serverID]
	if !exists {
		return nil
	}

	instances := make([]*PluginInstance, 0, len(serverPlugins))
	for _, instance := range serverPlugins {
		instances = append(instances, instance)
	}

	return instances
}

// GetPluginInstance returns a specific plugin instance
func (pm *PluginManager) GetPluginInstance(serverID, instanceID uuid.UUID) (*PluginInstance, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	serverPlugins, exists := pm.plugins[serverID]
	if !exists {
		return nil, fmt.Errorf("no plugins found for server %s", serverID.String())
	}

	instance, exists := serverPlugins[instanceID]
	if !exists {
		return nil, fmt.Errorf("plugin instance %s not found", instanceID.String())
	}

	return instance, nil
}

// DeletePluginInstance removes and stops a plugin instance
func (pm *PluginManager) DeletePluginInstance(serverID, instanceID uuid.UUID) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	serverPlugins, exists := pm.plugins[serverID]
	if !exists {
		return fmt.Errorf("no plugins found for server %s", serverID.String())
	}

	instance, exists := serverPlugins[instanceID]
	if !exists {
		return fmt.Errorf("plugin instance %s not found", instanceID.String())
	}

	// Stop plugin
	if err := pm.stopPluginInstance(instance); err != nil {
		log.Error().
			Str("serverID", serverID.String()).
			Str("instanceID", instanceID.String()).
			Err(err).
			Msg("Failed to stop plugin instance during deletion")
	}

	// Remove from database
	if err := pm.deletePluginInstanceFromDatabase(instanceID); err != nil {
		log.Error().
			Str("instanceID", instanceID.String()).
			Err(err).
			Msg("Failed to delete plugin instance from database")
	}

	// Remove from memory
	delete(serverPlugins, instanceID)
	if len(serverPlugins) == 0 {
		delete(pm.plugins, serverID)
	}

	log.Info().
		Str("serverID", serverID.String()).
		Str("instanceID", instanceID.String()).
		Str("pluginID", instance.PluginID).
		Msg("Deleted plugin instance")

	return nil
}

// UpdatePluginConfig updates a plugin's configuration
func (pm *PluginManager) UpdatePluginConfig(serverID, instanceID uuid.UUID, config map[string]interface{}) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	instance, err := pm.getPluginInstanceUnsafe(serverID, instanceID)
	if err != nil {
		return err
	}

	// Update plugin config
	if err := instance.Plugin.UpdateConfig(config); err != nil {
		return fmt.Errorf("failed to update plugin config: %w", err)
	}

	// Update instance record
	instance.Config = config
	instance.UpdatedAt = time.Now()

	// Save to database
	if err := pm.updatePluginInstanceInDatabase(instance); err != nil {
		return fmt.Errorf("failed to update plugin instance in database: %w", err)
	}

	return nil
}

// EnablePluginInstance enables a plugin instance
func (pm *PluginManager) EnablePluginInstance(serverID, instanceID uuid.UUID) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	instance, err := pm.getPluginInstanceUnsafe(serverID, instanceID)
	if err != nil {
		return err
	}

	if instance.Enabled {
		return nil // Already enabled
	}

	instance.Enabled = true
	instance.UpdatedAt = time.Now()

	// Initialize and start if needed
	if instance.Status == PluginStatusDisabled {
		if err := pm.initializePluginInstance(instance); err != nil {
			instance.Enabled = false
			return fmt.Errorf("failed to initialize plugin instance: %w", err)
		}
	}

	// Save to database
	if err := pm.updatePluginInstanceInDatabase(instance); err != nil {
		return fmt.Errorf("failed to update plugin instance in database: %w", err)
	}

	return nil
}

// DisablePluginInstance disables a plugin instance
func (pm *PluginManager) DisablePluginInstance(serverID, instanceID uuid.UUID) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	instance, err := pm.getPluginInstanceUnsafe(serverID, instanceID)
	if err != nil {
		return err
	}

	if !instance.Enabled {
		return nil // Already disabled
	}

	// Stop plugin
	if err := pm.stopPluginInstance(instance); err != nil {
		log.Error().
			Str("serverID", serverID.String()).
			Str("instanceID", instanceID.String()).
			Err(err).
			Msg("Failed to stop plugin instance during disable")
	}

	instance.Enabled = false
	instance.Status = PluginStatusDisabled
	instance.UpdatedAt = time.Now()

	// Save to database
	if err := pm.updatePluginInstanceInDatabase(instance); err != nil {
		return fmt.Errorf("failed to update plugin instance in database: %w", err)
	}

	return nil
}

// GetConnectorAPI returns a connector's API for use by plugins
func (pm *PluginManager) GetConnectorAPI(connectorID string) (interface{}, error) {
	pm.connectorMu.RLock()
	defer pm.connectorMu.RUnlock()

	instance, exists := pm.connectors[connectorID]
	if !exists {
		return nil, fmt.Errorf("connector %s not found", connectorID)
	}

	if instance.Status != ConnectorStatusRunning {
		return nil, fmt.Errorf("connector %s is not running", connectorID)
	}

	return instance.Connector.GetAPI(), nil
}

// ListAvailableConnectors returns a list of running connector IDs
func (pm *PluginManager) ListAvailableConnectors() []string {
	pm.connectorMu.RLock()
	defer pm.connectorMu.RUnlock()

	connectors := make([]string, 0, len(pm.connectors))
	for connectorID, instance := range pm.connectors {
		if instance.Status == ConnectorStatusRunning {
			connectors = append(connectors, connectorID)
		}
	}

	return connectors
}

// ListAvailablePlugins returns all available plugin definitions
func (pm *PluginManager) ListAvailablePlugins() []PluginDefinition {
	return pm.registry.ListPlugins()
}

// ListAvailableConnectorDefinitions returns all available connector definitions
func (pm *PluginManager) ListAvailableConnectorDefinitions() []ConnectorDefinition {
	return pm.connectorRegistry.ListConnectors()
}

// GetConnectors returns all connector instances
func (pm *PluginManager) GetConnectors() []*ConnectorInstance {
	pm.connectorMu.RLock()
	defer pm.connectorMu.RUnlock()

	connectors := make([]*ConnectorInstance, 0, len(pm.connectors))
	for _, instance := range pm.connectors {
		connectors = append(connectors, instance)
	}

	return connectors
}

// Private methods

func (pm *PluginManager) getPluginInstanceUnsafe(serverID, instanceID uuid.UUID) (*PluginInstance, error) {
	serverPlugins, exists := pm.plugins[serverID]
	if !exists {
		return nil, fmt.Errorf("no plugins found for server %s", serverID.String())
	}

	instance, exists := serverPlugins[instanceID]
	if !exists {
		return nil, fmt.Errorf("plugin instance %s not found", instanceID.String())
	}

	return instance, nil
}

func (pm *PluginManager) initializePluginInstance(instance *PluginInstance) error {
	instance.Status = PluginStatusStarting

	// Create plugin APIs
	apis := pm.createPluginAPIs(instance.ServerID, instance.ID)

	// Initialize plugin
	if err := instance.Plugin.Initialize(instance.Config, apis); err != nil {
		instance.Status = PluginStatusError
		instance.LastError = err.Error()
		return fmt.Errorf("failed to initialize plugin: %w", err)
	}

	// Start plugin if it's long-running
	definition := instance.Plugin.GetDefinition()
	if definition.LongRunning {
		if err := instance.Plugin.Start(instance.Context); err != nil {
			instance.Status = PluginStatusError
			instance.LastError = err.Error()
			return fmt.Errorf("failed to start plugin: %w", err)
		}
	}

	instance.Status = PluginStatusRunning
	instance.LastError = ""
	return nil
}

func (pm *PluginManager) stopPluginInstance(instance *PluginInstance) error {
	instance.Status = PluginStatusStopping

	// Cancel context
	if instance.Cancel != nil {
		instance.Cancel()
	}

	// Stop plugin
	if err := instance.Plugin.Stop(); err != nil {
		instance.Status = PluginStatusError
		instance.LastError = err.Error()
		return fmt.Errorf("failed to stop plugin: %w", err)
	}

	instance.Status = PluginStatusStopped
	return nil
}

func (pm *PluginManager) createPluginAPIs(serverID, instanceID uuid.UUID) *PluginAPIs {
	return &PluginAPIs{
		ServerAPI:    NewServerAPI(serverID, pm.db, pm.rconManager),
		DatabaseAPI:  NewDatabaseAPI(instanceID, pm.db),
		RconAPI:      NewRconAPI(serverID, pm.rconManager),
		EventAPI:     NewEventAPI(serverID, pm.eventManager),
		ConnectorAPI: NewConnectorAPI(pm),
		LogAPI:       NewLogAPI(serverID, instanceID),
	}
}

// Event distribution loop
func (pm *PluginManager) eventDistributionLoop() {
	for {
		select {
		case <-pm.ctx.Done():
			return
		case event := <-pm.eventSubscriber.Channel:
			pm.distributeEventToPlugins(&event)
		}
	}
}

func (pm *PluginManager) distributeEventToPlugins(event *event_manager.Event) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Convert event to plugin event
	rawString := ""
	if event.RawData != nil {
		if str, ok := event.RawData.(string); ok {
			rawString = str
		}
	}

	pluginEvent := &PluginEvent{
		ID:        event.ID,
		ServerID:  event.ServerID,
		Source:    pm.convertEventSource(event.Type),
		Type:      string(event.Type),
		Data:      event.Data,
		Raw:       rawString,
		Timestamp: event.Timestamp,
	}

	// Distribute to plugins on the specific server
	if serverPlugins, exists := pm.plugins[event.ServerID]; exists {
		for _, instance := range serverPlugins {
			if instance.Status != PluginStatusRunning || !instance.Enabled {
				continue
			}

			// Check if plugin handles this event type
			definition := instance.Plugin.GetDefinition()
			handles := false
			for _, handler := range definition.EventHandlers {
				if handler.EventType == string(event.Type) || handler.EventType == "*" {
					handles = true
					break
				}
			}

			if handles {
				go pm.handlePluginEvent(instance, pluginEvent)
			}
		}
	}
}

func (pm *PluginManager) handlePluginEvent(instance *PluginInstance, event *PluginEvent) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().
				Str("serverID", instance.ServerID.String()).
				Str("instanceID", instance.ID.String()).
				Str("pluginID", instance.PluginID).
				Interface("panic", r).
				Msg("Plugin panicked while handling event")

			instance.Status = PluginStatusError
			instance.LastError = fmt.Sprintf("panic: %v", r)
		}
	}()

	if err := instance.Plugin.HandleEvent(event); err != nil {
		log.Error().
			Str("serverID", instance.ServerID.String()).
			Str("instanceID", instance.ID.String()).
			Str("pluginID", instance.PluginID).
			Str("eventType", event.Type).
			Err(err).
			Msg("Plugin failed to handle event")

		instance.LastError = err.Error()
	}
}

func (pm *PluginManager) convertEventSource(eventType event_manager.EventType) EventSource {
	eventTypeStr := string(eventType)
	switch {
	case eventTypeStr[:4] == "RCON":
		return EventSourceRCON
	case eventTypeStr[:3] == "LOG":
		return EventSourceLog
	default:
		return EventSourceSystem
	}
}
