package extension_manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/connector_manager"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
)

// ExtensionHandler is a function that handles an event
type ExtensionHandler func(serverID uuid.UUID, eventType string, data interface{}) error

// Extension represents a loaded extension
type Extension interface {
	// GetID returns the unique identifier for this extension
	GetID() uuid.UUID
	// GetName returns the name of the extension
	GetName() string
	// GetDescription returns the description of the extension
	GetDescription() string
	// GetVersion returns the version of the extension
	GetVersion() string
	// GetAuthor returns the author of the extension
	GetAuthor() string
	// GetEventHandlers returns a map of event types to handlers
	GetEventHandlers() map[string]ExtensionHandler
	// GetRequiredConnectors returns a list of connector types required by this extension
	GetRequiredConnectors() []string
	// Initialize initializes the extension with its configuration and connectors
	Initialize(serverID uuid.UUID, config map[string]interface{}, connectors map[string]connector_manager.ConnectorInstance, rconManager *rcon_manager.RconManager) error
	// Shutdown gracefully shuts down the extension
	Shutdown() error
}

// ExtensionFactory creates extension instances
type ExtensionFactory interface {
	// Create creates a new extension instance
	Create() Extension
	// GetConfigSchema returns the configuration schema for this extension
	GetConfigSchema() map[string]interface{}
}

// ExtensionInstance represents an instantiated extension for a specific server
type ExtensionInstance struct {
	Extension Extension
	ServerID  uuid.UUID
	Enabled   bool
	Config    map[string]interface{}
}

// ExtensionManager manages all extensions
type ExtensionManager struct {
	factories        map[string]ExtensionFactory
	instances        map[uuid.UUID]ExtensionInstance
	serverExtensions map[uuid.UUID][]uuid.UUID // Maps server IDs to extension instance IDs
	connectorManager *connector_manager.ConnectorManager
	rconManager      *rcon_manager.RconManager
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewExtensionManager creates a new extension manager
func NewExtensionManager(ctx context.Context, connectorManager *connector_manager.ConnectorManager, rconManager *rcon_manager.RconManager) *ExtensionManager {
	ctx, cancel := context.WithCancel(ctx)
	return &ExtensionManager{
		factories:        make(map[string]ExtensionFactory),
		instances:        make(map[uuid.UUID]ExtensionInstance),
		serverExtensions: make(map[uuid.UUID][]uuid.UUID),
		connectorManager: connectorManager,
		rconManager:      rconManager,
		ctx:              ctx,
		cancel:           cancel,
	}
}

// RegisterFactory registers an extension factory
func (m *ExtensionManager) RegisterFactory(name string, factory ExtensionFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.factories[name] = factory
	log.Info().Str("name", name).Msg("Registered extension factory")
}

// ListFactories returns a map of all registered extension factories
func (m *ExtensionManager) ListFactories() map[string]ExtensionFactory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a copy of the factories map to avoid concurrent access issues
	factories := make(map[string]ExtensionFactory)
	for k, v := range m.factories {
		factories[k] = v
	}

	return factories
}

// GetFactory returns an extension factory by its name
func (m *ExtensionManager) GetFactory(name string) (ExtensionFactory, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	factory, ok := m.factories[name]
	return factory, ok
}

// InitializeExtensions initializes all extensions from the database
func (m *ExtensionManager) InitializeExtensions(ctx context.Context, db *sql.DB) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Query for server extensions
	rows, err := db.QueryContext(ctx, `
		SELECT id, server_id, name, enabled, config
		FROM server_extensions
	`)
	if err != nil {
		return fmt.Errorf("failed to query server extensions: %w", err)
	}
	defer rows.Close()

	// Initialize extensions
	for rows.Next() {
		var id uuid.UUID
		var serverID uuid.UUID
		var name string
		var enabled bool
		var configJSON []byte

		if err := rows.Scan(&id, &serverID, &name, &enabled, &configJSON); err != nil {
			log.Error().Err(err).Msg("Failed to scan server extension row")
			continue
		}

		var config map[string]interface{}
		if err := json.Unmarshal(configJSON, &config); err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to unmarshal extension config")
			continue
		}

		// Find factory for this extension
		factory, ok := m.factories[name]
		if !ok {
			log.Error().Str("name", name).Msg("No factory registered for extension")
			continue
		}

		// Create extension
		extension := factory.Create()

		// Get required connectors
		requiredConnectors := extension.GetRequiredConnectors()
		connectors := make(map[string]connector_manager.ConnectorInstance)

		// Get server connectors
		serverConnectors := m.connectorManager.GetConnectorsByServer(serverID)
		for _, connector := range serverConnectors {
			for _, required := range requiredConnectors {
				if connector.GetType() == required {
					connectors[required] = connector
					break
				}
			}
		}

		// Get global connectors for any missing required connectors
		for _, required := range requiredConnectors {
			if _, ok := connectors[required]; !ok {
				// Try to get from global connectors
				globalConnectors, err := m.connectorManager.GetConnectorsByType(required)
				if err != nil {
					log.Error().Err(err).Str("type", required).Msg("Failed to get global connectors")
					continue
				}
				if len(globalConnectors) > 0 {
					connectors[required] = globalConnectors[0]
				}
			}
		}

		// Check if we have all required connectors
		missingConnectors := false
		for _, required := range requiredConnectors {
			if _, ok := connectors[required]; !ok {
				log.Error().
					Str("extension", name).
					Str("serverID", serverID.String()).
					Str("connector", required).
					Msg("Missing required connector for extension")
				missingConnectors = true
			}
		}

		if missingConnectors {
			log.Error().
				Str("extension", name).
				Str("serverID", serverID.String()).
				Msg("Extension not loaded due to missing connectors")
			continue
		}

		// Skip disabled extensions
		if !enabled {
			// Still store the extension instance but don't initialize it
			instance := ExtensionInstance{
				Extension: extension,
				ServerID:  serverID,
				Enabled:   false,
				Config:    config,
			}

			m.instances[id] = instance

			// Add to server extensions map
			if _, ok := m.serverExtensions[serverID]; !ok {
				m.serverExtensions[serverID] = []uuid.UUID{}
			}
			m.serverExtensions[serverID] = append(m.serverExtensions[serverID], id)

			log.Info().
				Str("id", id.String()).
				Str("serverID", serverID.String()).
				Str("extension", name).
				Bool("enabled", false).
				Msg("Registered disabled extension")
			continue
		}

		// Initialize extension
		if err := extension.Initialize(serverID, config, connectors, m.rconManager); err != nil {
			log.Error().
				Err(err).
				Str("extension", name).
				Str("serverID", serverID.String()).
				Msg("Failed to initialize extension")
			continue
		}

		// Store extension instance
		instance := ExtensionInstance{
			Extension: extension,
			ServerID:  serverID,
			Enabled:   enabled,
			Config:    config,
		}

		m.instances[id] = instance

		// Add to server extensions map
		if _, ok := m.serverExtensions[serverID]; !ok {
			m.serverExtensions[serverID] = []uuid.UUID{}
		}
		m.serverExtensions[serverID] = append(m.serverExtensions[serverID], id)

		log.Info().
			Str("id", id.String()).
			Str("serverID", serverID.String()).
			Str("extension", name).
			Bool("enabled", enabled).
			Msg("Initialized extension")
	}

	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating extension rows")
	}

	return nil
}

// InitializeExtension initializes a specific extension
func (m *ExtensionManager) InitializeExtension(id uuid.UUID, serverID uuid.UUID, name string, config map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if extension already exists and is enabled
	if instance, ok := m.instances[id]; ok && instance.Enabled {
		// Extension already initialized
		return nil
	}

	// Find factory for this extension
	factory, ok := m.factories[name]
	if !ok {
		return fmt.Errorf("no factory registered for extension: %s", name)
	}

	// Create extension
	extension := factory.Create()

	// Get required connectors
	requiredConnectors := extension.GetRequiredConnectors()
	connectors := make(map[string]connector_manager.ConnectorInstance)

	// Get server connectors
	serverConnectors := m.connectorManager.GetConnectorsByServer(serverID)
	for _, connector := range serverConnectors {
		for _, required := range requiredConnectors {
			if connector.GetType() == required {
				connectors[required] = connector
				break
			}
		}
	}

	// Get global connectors for any missing required connectors
	for _, required := range requiredConnectors {
		if _, ok := connectors[required]; !ok {
			// Try to get from global connectors
			globalConnectors, err := m.connectorManager.GetConnectorsByType(required)
			if err != nil {
				log.Error().Err(err).Str("type", required).Msg("Failed to get global connectors")
				continue
			}
			if len(globalConnectors) > 0 {
				connectors[required] = globalConnectors[0]
			}
		}
	}

	// Check if we have all required connectors
	missingConnectors := false
	for _, required := range requiredConnectors {
		if _, ok := connectors[required]; !ok {
			log.Error().
				Str("extension", name).
				Str("serverID", serverID.String()).
				Str("connector", required).
				Msg("Missing required connector for extension")
			missingConnectors = true
		}
	}

	if missingConnectors {
		return fmt.Errorf("extension not loaded due to missing connectors")
	}

	// Initialize extension
	if err := extension.Initialize(serverID, config, connectors, m.rconManager); err != nil {
		return fmt.Errorf("failed to initialize extension: %w", err)
	}

	// Store extension instance
	instance := ExtensionInstance{
		Extension: extension,
		ServerID:  serverID,
		Enabled:   true,
		Config:    config,
	}

	m.instances[id] = instance

	// Add to server extensions map
	if _, ok := m.serverExtensions[serverID]; !ok {
		m.serverExtensions[serverID] = []uuid.UUID{}
	}

	// Check if it's already in the list
	found := false
	for _, existingID := range m.serverExtensions[serverID] {
		if existingID == id {
			found = true
			break
		}
	}

	// Only add if not already in the list
	if !found {
		m.serverExtensions[serverID] = append(m.serverExtensions[serverID], id)
	}

	log.Info().
		Str("id", id.String()).
		Str("serverID", serverID.String()).
		Str("extension", name).
		Msg("Initialized extension")

	return nil
}

// ShutdownExtension shuts down a specific extension
func (m *ExtensionManager) ShutdownExtension(id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if extension exists
	instance, ok := m.instances[id]
	if !ok {
		// Not found, nothing to do
		return nil
	}

	// Only shutdown if it's enabled
	if instance.Enabled {
		if err := instance.Extension.Shutdown(); err != nil {
			return fmt.Errorf("failed to shutdown extension: %w", err)
		}

		// Mark as disabled
		instance.Enabled = false
		m.instances[id] = instance
	}

	log.Info().
		Str("id", id.String()).
		Str("serverID", instance.ServerID.String()).
		Str("extension", instance.Extension.GetName()).
		Msg("Shutdown extension")

	return nil
}

// RestartExtension restarts an extension with new configuration
func (m *ExtensionManager) RestartExtension(id uuid.UUID, config map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get the existing instance
	instance, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("extension not found: %s", id)
	}

	// If it's not enabled, just update the config
	if !instance.Enabled {
		instance.Config = config
		m.instances[id] = instance
		return nil
	}

	// Shutdown the existing instance
	if err := instance.Extension.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown extension: %w", err)
	}

	// Find factory for this extension
	name := instance.Extension.GetName()
	factory, ok := m.factories[name]
	if !ok {
		return fmt.Errorf("no factory registered for extension: %s", name)
	}

	// Create new extension
	extension := factory.Create()

	// Get required connectors
	requiredConnectors := extension.GetRequiredConnectors()
	connectors := make(map[string]connector_manager.ConnectorInstance)

	// Get server connectors
	serverConnectors := m.connectorManager.GetConnectorsByServer(instance.ServerID)
	for _, connector := range serverConnectors {
		for _, required := range requiredConnectors {
			if connector.GetType() == required {
				connectors[required] = connector
				break
			}
		}
	}

	// Get global connectors for any missing required connectors
	for _, required := range requiredConnectors {
		if _, ok := connectors[required]; !ok {
			// Try to get from global connectors
			globalConnectors, err := m.connectorManager.GetConnectorsByType(required)
			if err != nil {
				log.Error().Err(err).Str("type", required).Msg("Failed to get global connectors")
				continue
			}
			if len(globalConnectors) > 0 {
				connectors[required] = globalConnectors[0]
			}
		}
	}

	// Check if we have all required connectors
	missingConnectors := false
	for _, required := range requiredConnectors {
		if _, ok := connectors[required]; !ok {
			log.Error().
				Str("extension", name).
				Str("serverID", instance.ServerID.String()).
				Str("connector", required).
				Msg("Missing required connector for extension")
			missingConnectors = true
		}
	}

	if missingConnectors {
		return fmt.Errorf("extension not restarted due to missing connectors")
	}

	// Initialize extension with new config
	if err := extension.Initialize(instance.ServerID, config, connectors, m.rconManager); err != nil {
		return fmt.Errorf("failed to initialize extension: %w", err)
	}

	// Store updated instance
	m.instances[id] = ExtensionInstance{
		Extension: extension,
		ServerID:  instance.ServerID,
		Enabled:   true,
		Config:    config,
	}

	log.Info().
		Str("id", id.String()).
		Str("serverID", instance.ServerID.String()).
		Str("extension", name).
		Msg("Restarted extension")

	return nil
}

// HandleEvent dispatches an event to all relevant extensions
func (m *ExtensionManager) HandleEvent(serverID uuid.UUID, eventType string, data interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get all extensions for this server
	extensionIDs, ok := m.serverExtensions[serverID]
	if !ok {
		return
	}

	// Process event for each extension
	for _, id := range extensionIDs {
		instance, ok := m.instances[id]
		if !ok || !instance.Enabled {
			continue
		}

		// Get event handlers for this extension
		handlers := instance.Extension.GetEventHandlers()
		handler, ok := handlers[eventType]
		if !ok {
			continue
		}

		// Handle event in a separate goroutine to avoid blocking
		go func(ext Extension, handler ExtensionHandler, sid uuid.UUID, etype string, eventData interface{}) {
			if err := handler(sid, etype, eventData); err != nil {
				log.Error().
					Err(err).
					Str("extension", ext.GetName()).
					Str("serverID", sid.String()).
					Str("eventType", etype).
					Msg("Error handling event")
			}
		}(instance.Extension, handler, serverID, eventType, data)
	}
}

// StartEventListener starts listening for RCON events and dispatches them to extensions
func (m *ExtensionManager) StartEventListener() {
	// Subscribe to RCON events
	eventChan := m.rconManager.SubscribeToEvents()

	// Start event listener
	go func() {
		for {
			select {
			case <-m.ctx.Done():
				m.rconManager.UnsubscribeFromEvents(eventChan)
				return
			case event := <-eventChan:
				// Process event
				m.HandleEvent(event.ServerID, event.Type, event.Data)
			}
		}
	}()

	log.Info().Msg("Extension event listener started")
}

// Shutdown shuts down all extensions
func (m *ExtensionManager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel context
	m.cancel()

	// Shutdown all extension instances
	for id, instance := range m.instances {
		if instance.Enabled {
			if err := instance.Extension.Shutdown(); err != nil {
				log.Error().
					Err(err).
					Str("id", id.String()).
					Str("extension", instance.Extension.GetName()).
					Msg("Error shutting down extension")
			}
		}
	}

	log.Info().Msg("All extensions shut down")
}
