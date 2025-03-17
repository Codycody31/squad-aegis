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

// startEventListener subscribes to RCON events and dispatches them to extensions
// TODO: Seperate rcon event listener and the log event listener
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
		Connectors: make(map[string]connector_manager.Connector),
	}

	// Check required dependencies
	for _, depType := range def.Dependencies.Required {
		switch depType {
		case DependencyDatabase:
			if m.db == nil {
				return nil, fmt.Errorf("required dependency not available: database")
			}

			deps.Database = m.db
		case DependencyServer:
			if server == nil {
				return nil, fmt.Errorf("required dependency not available: server")
			}

			deps.Server = server
		case DependencyRconManager:
			if m.rconManager == nil {
				return nil, fmt.Errorf("required dependency not available: rcon_manager")
			}

			deps.RconManager = m.rconManager
		case DependencyConnectors:
			// Get server connectors (including global connectors)
			serverConnectors := m.connectorManager.GetConnectorsByServer(server.Id)

			// Add connectors to dependencies
			for _, requiredConnector := range def.RequiredConnectors {
				for _, connector := range serverConnectors {
					if connector.GetType() == requiredConnector {
						deps.Connectors[connector.GetType()] = connector
						break
					}
				}

				// Check if required connector is available
				if _, exists := deps.Connectors[requiredConnector]; !exists {
					return nil, fmt.Errorf("required connector not found: %s", requiredConnector)
				}
			}

			// Add optional connectors
			for _, optionalConnector := range def.OptionalConnectors {
				for _, connector := range serverConnectors {
					if connector.GetType() == optionalConnector {
						deps.Connectors[connector.GetType()] = connector
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

// InitializeExtensions initializes all extensions from the database
func (m *ExtensionManager) InitializeExtensions(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Query for server extensions
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, server_id, name, enabled, config
		FROM server_extensions
	`)
	if err != nil {
		return fmt.Errorf("failed to query server extensions: %w", err)
	}
	defer rows.Close()

	// Track which extensions are used by each server
	serverExtensionTypes := make(map[uuid.UUID]map[string]bool)

	// Initialize extensions
	for rows.Next() {
		var dbID string
		var serverID uuid.UUID
		var name string
		var enabled bool
		var configJSON []byte

		if err := rows.Scan(&dbID, &serverID, &name, &enabled, &configJSON); err != nil {
			log.Error().Err(err).Msg("Failed to scan server extension row")
			continue
		}

		var config map[string]interface{}
		if err := json.Unmarshal(configJSON, &config); err != nil {
			log.Error().Err(err).Str("name", name).Msg("Failed to unmarshal extension config")
			continue
		}

		// Find registrar for this extension
		registrar, ok := m.registeredExtensions[name]
		if !ok {
			log.Error().Str("name", name).Msg("No registrar found for extension")
			continue
		}

		// Get extension definition
		def := registrar.Define()

		// Check if this extension type is already in use for this server and doesn't allow multiple instances
		if !def.AllowMultipleInstances {
			if _, exists := serverExtensionTypes[serverID]; !exists {
				serverExtensionTypes[serverID] = make(map[string]bool)
			}

			if serverExtensionTypes[serverID][def.ID] {
				log.Warn().
					Str("extension", name).
					Str("serverID", serverID.String()).
					Msg("Extension doesn't allow multiple instances and is already in use for this server. Skipping.")
				continue
			}

			// Mark this extension type as in use for this server
			serverExtensionTypes[serverID][def.ID] = true
		}

		// Use the ID provided by the extension
		extensionID := def.ID

		// Get server
		server, err := core.GetServerById(ctx, m.db, serverID, nil)
		if err != nil {
			log.Error().Err(err).Str("serverID", serverID.String()).Msg("Failed to get server")
			continue
		}

		// Create dependencies
		deps, err := m.createDependencies(def, server)
		if err != nil {
			log.Error().
				Err(err).
				Str("extension", name).
				Str("serverID", serverID.String()).
				Msg("Failed to create dependencies")
			continue
		}

		// Create extension instance
		instance := def.CreateInstance()

		// Skip disabled extensions
		if !enabled {
			// Store the extension instance but don't initialize it
			m.instances[extensionID] = instance

			// Add to server extensions map
			if _, ok := m.serverExtensions[serverID]; !ok {
				m.serverExtensions[serverID] = []string{}
			}
			m.serverExtensions[serverID] = append(m.serverExtensions[serverID], extensionID)

			log.Info().
				Str("id", extensionID).
				Str("serverID", serverID.String()).
				Str("extension", name).
				Bool("enabled", false).
				Msg("Registered disabled extension")
			continue
		}

		// Initialize extension
		if err := instance.Initialize(config, deps); err != nil {
			log.Error().
				Err(err).
				Str("extension", name).
				Str("serverID", serverID.String()).
				Msg("Failed to initialize extension")
			continue
		}

		// Store extension instance
		m.instances[extensionID] = instance

		// Add to server extensions map
		if _, ok := m.serverExtensions[serverID]; !ok {
			m.serverExtensions[serverID] = []string{}
		}
		m.serverExtensions[serverID] = append(m.serverExtensions[serverID], extensionID)

		log.Info().
			Str("id", extensionID).
			Str("serverID", serverID.String()).
			Str("extension", name).
			Bool("enabled", enabled).
			Msg("Initialized extension")
	}

	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating extension rows")
	}

	// Start the event listener after initializing extensions
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
