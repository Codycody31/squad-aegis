package extension_manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/core"
	"go.codycody31.dev/squad-aegis/internal/connector_manager"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
)

// ExtensionHandler is a function that handles an event
type ExtensionHandler func(serverID uuid.UUID, eventType string, data interface{}) error

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
	Server    *models.Server
	Enabled   bool
	Config    map[string]interface{}
}

// ExtensionManager manages all extensions
type ExtensionManager struct {
	registeredExtensions map[string]ExtensionRegistrar
	instances            map[string]Extension
	serverExtensions     map[uuid.UUID][]string // Maps server IDs to extension instance IDs
	connectorManager     *connector_manager.ConnectorManager
	rconManager          *rcon_manager.RconManager
	db                   *sql.DB
	mu                   sync.RWMutex
	ctx                  context.Context
	cancel               context.CancelFunc
}

// NewExtensionManager creates a new extension manager
func NewExtensionManager(ctx context.Context, db *sql.DB, connectorManager *connector_manager.ConnectorManager, rconManager *rcon_manager.RconManager) *ExtensionManager {
	ctx, cancel := context.WithCancel(ctx)
	return &ExtensionManager{
		registeredExtensions: make(map[string]ExtensionRegistrar),
		instances:            make(map[string]Extension),
		serverExtensions:     make(map[uuid.UUID][]string),
		db:                   db,
		connectorManager:     connectorManager,
		rconManager:          rconManager,
		ctx:                  ctx,
		cancel:               cancel,
	}
}

// HandleConnectorEvent handles events from connectors
func (m *ExtensionManager) HandleConnectorEvent(event connector_manager.Event) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get extensions for this server
	extensionIDs, ok := m.serverExtensions[event.ServerID]
	if !ok {
		// No extensions for this server
		return nil
	}

	// Get the connector instance to determine its scope
	connector, exists := m.connectorManager.GetConnectorByID(uuid.MustParse(event.ConnectorID))
	if !exists {
		log.Error().Str("connectorID", event.ConnectorID).Msg("Connector not found")
		return nil
	}

	def := connector.GetDefinition()

	// Dispatch event to all extensions for this server
	for _, extensionID := range extensionIDs {
		extension, ok := m.instances[extensionID]
		if !ok {
			continue
		}

		// Get extension definition
		extDef := extension.GetDefinition()

		// Check if extension requires this connector
		requiresConnector := false
		for _, reqConnector := range extDef.RequiredConnectors {
			if reqConnector == def.ID {
				requiresConnector = true
				break
			}
		}

		// Also check optional connectors
		if !requiresConnector {
			for _, optConnector := range extDef.OptionalConnectors {
				if optConnector == def.ID {
					requiresConnector = true
					break
				}
			}
		}

		if !requiresConnector {
			continue
		}

		// Check if extension has handlers for this event source
		for _, handler := range extDef.EventHandlers {
			if handler.Source == EventHandlerSourceCONNECTOR && handler.Name == event.Type {
				// Handle event in a goroutine to prevent blocking
				go func(h EventHandler) {
					if err := h.Handler(extension, event.Data); err != nil {
						log.Error().
							Err(err).
							Str("extensionID", extensionID).
							Str("connectorID", event.ConnectorID).
							Str("eventType", event.Type).
							Msg("Error handling connector event")
					}
				}(handler)
			}
		}
	}

	return nil
}

// startEventListener subscribes to RCON events and dispatches them to extensions
func (m *ExtensionManager) startEventListener() {
	if m.rconManager == nil {
		log.Warn().Msg("RCON manager not available, cannot listen for events")
		return
	}

	// Subscribe to RCON events
	eventChan := m.rconManager.SubscribeToEvents()

	// Start a goroutine to process events
	go func() {
		for {
			select {
			case <-m.ctx.Done():
				m.rconManager.UnsubscribeFromEvents(eventChan)
				log.Info().Msg("Extension manager event listener stopped")
				return
			case event := <-eventChan:
				// Map RCON event to extension event
				m.HandleEvent(event.ServerID, event.Type, event.Data)
			}
		}
	}()

	log.Info().Msg("Extension manager event listener started")
}

// createDependencies creates a Dependencies instance for an extension
func (m *ExtensionManager) createDependencies(def ExtensionDefinition, server *models.Server) (*Dependencies, error) {
	deps := &Dependencies{
		Database:    m.db,
		Server:      server,
		RconManager: m.rconManager,
		Connectors:  make(map[string]connector_manager.Connector),
	}

	// Check required dependencies
	for _, depType := range def.Dependencies.Required {
		switch depType {
		case DependencyDatabase:
			if deps.Database == nil {
				return nil, fmt.Errorf("required dependency not available: database")
			}
		case DependencyServer:
			if deps.Server == nil {
				return nil, fmt.Errorf("required dependency not available: server")
			}
		case DependencyRconManager:
			if deps.RconManager == nil {
				return nil, fmt.Errorf("required dependency not available: rcon_manager")
			}
		case DependencyConnectors:
			// Get server connectors (including global connectors)
			serverConnectors := m.connectorManager.GetConnectorsByServer(server.Id)

			// Add required connectors
			for _, requiredConnector := range def.RequiredConnectors {
				found := false
				for _, connector := range serverConnectors {
					connDef := connector.GetDefinition()
					if connDef.ID == requiredConnector {
						deps.Connectors[connDef.ID] = connector
						found = true
						break
					}
				}
				if !found {
					return nil, fmt.Errorf("required connector not found: %s", requiredConnector)
				}
			}

			// Add optional connectors if available
			for _, optionalConnector := range def.OptionalConnectors {
				for _, connector := range serverConnectors {
					connDef := connector.GetDefinition()
					if connDef.ID == optionalConnector {
						deps.Connectors[connDef.ID] = connector
						break
					}
				}
			}
		}
	}

	return deps, nil
}

// RegisterExtension registers an extension
func (m *ExtensionManager) RegisterExtension(name string, registrar ExtensionRegistrar) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.registeredExtensions[name] = registrar
	log.Info().Str("name", name).Msg("Registered extension")
}

// ListExtensions returns a list of all registered extensions and their definitions
func (m *ExtensionManager) ListExtensions() []ExtensionDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	definitions := make([]ExtensionDefinition, 0, len(m.registeredExtensions))
	for _, registrar := range m.registeredExtensions {
		definitions = append(definitions, registrar.Define())
	}

	return definitions
}

// GetExtension returns an extension registrar by its name
func (m *ExtensionManager) GetExtension(name string) (ExtensionRegistrar, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	registrar, ok := m.registeredExtensions[name]
	return registrar, ok
}

// Initialize initializes all extensions from the database
func (m *ExtensionManager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Subscribe to connector events if connector manager is available
	if m.connectorManager != nil {
		// Get all servers from the database
		rows, err := m.db.QueryContext(ctx, `
			SELECT id
			FROM servers
		`)
		if err != nil {
			return fmt.Errorf("failed to query servers: %w", err)
		}
		defer rows.Close()

		// For each server, get its connectors and subscribe to their events
		for rows.Next() {
			var serverID uuid.UUID
			if err := rows.Scan(&serverID); err != nil {
				log.Error().Err(err).Msg("Failed to scan server row")
				continue
			}

			// Get server connectors
			serverConnectors := m.connectorManager.GetConnectorsByServer(serverID)
			for _, connector := range serverConnectors {
				def := connector.GetDefinition()
				if def.Flags.ImplementsEvents {
					// Subscribe to server-specific connector events
					m.connectorManager.SubscribeToServerEvents(serverID, def.ID, func(event connector_manager.Event) error {
						return m.HandleConnectorEvent(event)
					})
					log.Info().
						Str("connectorID", def.ID).
						Str("serverID", serverID.String()).
						Msg("Subscribed to server connector events")
				}
			}
		}

		if err := rows.Err(); err != nil {
			log.Error().Err(err).Msg("Error iterating server rows")
		}

		// Subscribe to global connector events
		for _, registrar := range m.connectorManager.ListRegisteredConnectors() {
			def := registrar.Define()
			if def.Flags.ImplementsEvents && def.Scope == connector_manager.ConnectorScopeGlobal {
				m.connectorManager.SubscribeToEvents(def.ID, func(event connector_manager.Event) error {
					return m.HandleConnectorEvent(event)
				})
				log.Info().
					Str("connectorID", def.ID).
					Msg("Subscribed to global connector events")
			}
		}
	}

	// Query for server extensions
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, server_id, name, enabled, config
		FROM server_extensions
	`)
	if err != nil {
		return fmt.Errorf("failed to query server extensions: %w", err)
	}
	defer rows.Close()

	// Initialize server extensions
	for rows.Next() {
		var id uuid.UUID
		var serverID uuid.UUID
		var name string
		var enabled bool
		var configJSON []byte

		if err := rows.Scan(&id, &serverID, &name, &enabled, &configJSON); err != nil {
			log.Error().Err(err).Msg("Failed to scan extension row")
			continue
		}

		// Skip disabled extensions
		if !enabled {
			continue
		}

		// Get server
		server, err := core.GetServerById(ctx, m.db, serverID, nil)
		if err != nil {
			log.Error().Err(err).Str("serverID", serverID.String()).Msg("Failed to get server for extension")
			continue
		}

		// Find registrar for this extension type
		registrar, ok := m.registeredExtensions[name]
		if !ok {
			log.Error().Str("name", name).Msg("No registrar found for extension type")
			continue
		}

		// Get extension definition
		def := registrar.Define()

		// Parse config JSON
		var config map[string]interface{}
		if err := json.Unmarshal(configJSON, &config); err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to unmarshal extension config")
			continue
		}

		// Create dependencies
		deps, err := m.createDependencies(def, server)
		if err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to create extension dependencies")
			continue
		}

		// Create and initialize extension instance
		instance := def.CreateInstance()
		if err := instance.Initialize(config, deps); err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to initialize extension instance")
			continue
		}

		// Store instance
		m.instances[id.String()] = instance

		// Map server to extension
		if _, ok := m.serverExtensions[serverID]; !ok {
			m.serverExtensions[serverID] = make([]string, 0)
		}
		m.serverExtensions[serverID] = append(m.serverExtensions[serverID], id.String())

		log.Info().
			Str("id", id.String()).
			Str("serverID", serverID.String()).
			Str("type", def.ID).
			Msg("Initialized extension")
	}

	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating extension rows")
	}

	// Start event listener for RCON events
	m.startEventListener()

	return nil
}

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
		if !ok {
			continue
		}

		def := instance.GetDefinition()

		// Find matching event handlers
		for _, handler := range def.EventHandlers {
			if handler.Name == eventType {
				// Capture values inside the goroutine to prevent race conditions
				handlerCopy := handler
				instanceCopy := instance

				go func() {
					// 🔹 Call the method on the initialized extension instance
					if err := handlerCopy.Handler(instanceCopy, data); err != nil {
						log.Error().
							Err(err).
							Str("extension", def.Name).
							Str("handler", handlerCopy.Name).
							Str("description", handlerCopy.Description).
							Msg("Error handling event")
					}
				}()
			}
		}
	}
}

// Shutdown shuts down all extensions
func (m *ExtensionManager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel context
	m.cancel()

	// Shutdown all extension instances
	for id, instance := range m.instances {
		if err := instance.Shutdown(); err != nil {
			def := instance.GetDefinition()
			log.Error().
				Err(err).
				Str("id", id).
				Str("extension", def.Name).
				Msg("Error shutting down extension")
		}
	}

	log.Info().Msg("All extensions shut down")
}

// ShutdownExtension shuts down a specific extension for a server
func (m *ExtensionManager) ShutdownExtension(serverID uuid.UUID, extensionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get all extensions for this server
	extensionIDs, ok := m.serverExtensions[serverID]
	if !ok {
		return fmt.Errorf("no extensions found for server %s", serverID.String())
	}

	// Find the extension instance
	for _, id := range extensionIDs {
		if id == extensionID {
			instance, instanceOk := m.instances[id]
			if !instanceOk {
				return fmt.Errorf("extension instance not found for ID: %s", id)
			}

			// Shutdown the extension
			def := instance.GetDefinition()
			if err := instance.Shutdown(); err != nil {
				log.Error().
					Err(err).
					Str("id", id).
					Str("extension", def.Name).
					Str("serverID", serverID.String()).
					Msg("Error shutting down extension")
				return err
			}

			log.Info().
				Str("id", id).
				Str("extension", def.Name).
				Str("serverID", serverID.String()).
				Msg("Extension successfully shut down")

			return nil
		}
	}

	return fmt.Errorf("extension with ID %s not found for server %s", extensionID, serverID.String())
}

// EnableExtension enables or reinitializes an extension for a specific server
func (m *ExtensionManager) EnableExtension(ctx context.Context, serverID uuid.UUID, extensionName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Query for the specific extension
	var dbID string
	var enabled bool
	var configJSON []byte

	err := m.db.QueryRowContext(ctx, `
		SELECT id, enabled, config
		FROM server_extensions
		WHERE server_id = $1 AND name = $2
	`, serverID, extensionName).Scan(&dbID, &enabled, &configJSON)

	if err != nil {
		return fmt.Errorf("failed to query server extension: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return fmt.Errorf("failed to unmarshal extension config: %w", err)
	}

	// Find registrar for this extension
	registrar, ok := m.registeredExtensions[extensionName]
	if !ok {
		return fmt.Errorf("no registrar found for extension: %s", extensionName)
	}

	// Get extension definition
	def := registrar.Define()
	extensionID := def.ID

	// Get server
	server, err := core.GetServerById(ctx, m.db, serverID, nil)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}

	// Check if this extension is already initialized and enabled
	if instance, exists := m.instances[extensionID]; exists {
		// Shutdown the existing instance first
		if err := instance.Shutdown(); err != nil {
			log.Warn().
				Err(err).
				Str("extension", extensionName).
				Str("serverID", serverID.String()).
				Msg("Error shutting down existing extension instance")
		}

		// Remove from instances
		delete(m.instances, extensionID)
	}

	// Create dependencies
	deps, err := m.createDependencies(def, server)
	if err != nil {
		return fmt.Errorf("failed to create dependencies: %w", err)
	}

	// Create extension instance
	instance := def.CreateInstance()

	// Initialize extension
	if err := instance.Initialize(config, deps); err != nil {
		return fmt.Errorf("failed to initialize extension: %w", err)
	}

	// Store extension instance
	m.instances[extensionID] = instance

	// Add to server extensions map if not already there
	if _, ok := m.serverExtensions[serverID]; !ok {
		m.serverExtensions[serverID] = []string{}
	}

	// Check if the extension ID is already in the server extensions list
	found := false
	for _, id := range m.serverExtensions[serverID] {
		if id == extensionID {
			found = true
			break
		}
	}

	if !found {
		m.serverExtensions[serverID] = append(m.serverExtensions[serverID], extensionID)
	}

	log.Info().
		Str("id", extensionID).
		Str("serverID", serverID.String()).
		Str("extension", extensionName).
		Bool("enabled", true).
		Msg("Extension enabled and initialized")

	return nil
}
