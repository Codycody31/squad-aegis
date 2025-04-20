package connector_manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ConnectorManager manages all connectors
type ConnectorManager struct {
	registeredConnectors   map[string]ConnectorRegistrar
	instances              map[uuid.UUID]connectorInstance
	serverConnectors       map[uuid.UUID][]string                     // Maps server IDs to connector instance IDs
	eventSubscribers       map[string][]EventSubscriber               // Global event subscribers
	serverEventSubscribers map[uuid.UUID]map[string][]EventSubscriber // Server-specific event subscribers
	globalConfig           map[string]any
	subscriberCallbacks    map[string]map[string]func(data any)
	mu                     sync.RWMutex
	ctx                    context.Context
	cancel                 context.CancelFunc
}

// connectorInstance wraps a Connector with its metadata
type connectorInstance struct {
	connector Connector
	id        uuid.UUID
	config    map[string]any
	serverID  *uuid.UUID // nil for global connectors
}

// EventSubscriber represents a subscriber to connector events
type EventSubscriber struct {
	ID       string
	Callback func(event Event) error
}

// Event represents a connector event
type Event struct {
	ConnectorID string
	ServerID    uuid.UUID
	Type        string
	Data        any
}

// NewConnectorManager creates a new connector manager
func NewConnectorManager(ctx context.Context) *ConnectorManager {
	ctx, cancel := context.WithCancel(ctx)
	return &ConnectorManager{
		registeredConnectors:   make(map[string]ConnectorRegistrar),
		instances:              make(map[uuid.UUID]connectorInstance),
		serverConnectors:       make(map[uuid.UUID][]string),
		eventSubscribers:       make(map[string][]EventSubscriber),
		serverEventSubscribers: make(map[uuid.UUID]map[string][]EventSubscriber),
		globalConfig:           make(map[string]any),
		subscriberCallbacks:    make(map[string]map[string]func(data any)),
		ctx:                    ctx,
		cancel:                 cancel,
	}
}

// RegisterConnector registers a connector type
func (m *ConnectorManager) RegisterConnector(name string, registrar ConnectorRegistrar) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.registeredConnectors[name] = registrar
	log.Info().Str("name", name).Msg("Registered connector type")
}

// ListConnectors returns a list of all registered connector types and their definitions
func (m *ConnectorManager) ListConnectors() []ConnectorDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	definitions := make([]ConnectorDefinition, 0, len(m.registeredConnectors))
	for _, registrar := range m.registeredConnectors {
		definitions = append(definitions, registrar.Define())
	}

	return definitions
}

// GetConnector returns a connector registrar by its name
func (m *ConnectorManager) GetConnector(name string) (ConnectorRegistrar, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	registrar, ok := m.registeredConnectors[name]
	return registrar, ok
}

// InitializeConnectors initializes all connectors from the database
func (m *ConnectorManager) InitializeConnectors(ctx context.Context, db *sql.DB) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Debug().Msg("Starting connector initialization...")

	// Query for global connectors
	log.Debug().Msg("Querying global connectors...")
	globalRows, err := db.QueryContext(ctx, `
		SELECT id, name, config
		FROM connectors
	`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query global connectors")
		return fmt.Errorf("failed to query connectors: %w", err)
	}
	defer globalRows.Close()
	log.Debug().Msg("Global connectors query successful.")

	// Initialize global connectors
	globalCount := 0
	for globalRows.Next() {
		globalCount++
		var id uuid.UUID
		var name string
		var configJSON []byte

		if err := globalRows.Scan(&id, &name, &configJSON); err != nil {
			log.Error().Err(err).Int("globalIndex", globalCount).Msg("Failed to scan global connector row")
			continue
		}
		log.Trace().Str("id", id.String()).Str("name", name).Int("globalIndex", globalCount).Msg("Processing global connector")

		var config map[string]any
		if err := json.Unmarshal(configJSON, &config); err != nil {
			log.Error().Err(err).Str("id", id.String()).Int("globalIndex", globalCount).Msg("Failed to unmarshal global connector config")
			continue
		}

		// Find registrar for this connector type
		registrar, ok := m.registeredConnectors[name]
		if !ok {
			log.Error().Str("name", name).Str("id", id.String()).Int("globalIndex", globalCount).Msg("No registrar found for global connector type")
			continue
		}

		// Get connector definition
		def := registrar.Define()
		log.Trace().Str("id", id.String()).Str("type", def.ID).Int("globalIndex", globalCount).Msg("Creating global connector instance")

		// Create and initialize connector instance
		instance := def.CreateInstance()
		if err := instance.Initialize(config); err != nil {
			log.Error().Err(err).Str("id", id.String()).Str("type", def.ID).Int("globalIndex", globalCount).Msg("Failed to initialize global connector instance")
			continue
		}

		m.instances[id] = connectorInstance{
			connector: instance,
			id:        id,
			config:    config,
		}
		log.Info().Str("id", id.String()).Str("type", def.ID).Msg("Initialized global connector")
	}

	if err := globalRows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating global connector rows")
	}
	log.Debug().Int("count", globalCount).Msg("Finished processing global connectors.")

	// Query for server-specific connectors
	log.Debug().Msg("Querying server connectors...")
	serverRows, err := db.QueryContext(ctx, `
		SELECT id, server_id, name, config
		FROM server_connectors
	`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query server connectors")
		return fmt.Errorf("failed to query server connectors: %w", err)
	}
	defer serverRows.Close()
	log.Debug().Msg("Server connectors query successful.")

	// Initialize server-specific connectors
	serverCount := 0
	for serverRows.Next() {
		serverCount++
		var id uuid.UUID
		var serverID uuid.UUID
		var name string
		var configJSON []byte

		if err := serverRows.Scan(&id, &serverID, &name, &configJSON); err != nil {
			log.Error().Err(err).Int("serverIndex", serverCount).Msg("Failed to scan server connector row")
			continue
		}
		log.Trace().Str("id", id.String()).Str("serverID", serverID.String()).Str("name", name).Int("serverIndex", serverCount).Msg("Processing server connector")

		var config map[string]any
		if err := json.Unmarshal(configJSON, &config); err != nil {
			log.Error().Err(err).Str("id", id.String()).Int("serverIndex", serverCount).Msg("Failed to unmarshal server connector config")
			continue
		}

		// Add server ID to config
		config["server_id"] = serverID.String()

		// Find registrar for this connector type
		registrar, ok := m.registeredConnectors[name]
		if !ok {
			log.Error().Str("name", name).Str("id", id.String()).Int("serverIndex", serverCount).Msg("No registrar found for server connector type")
			continue
		}

		// Get connector definition
		def := registrar.Define()
		log.Trace().Str("id", id.String()).Str("type", def.ID).Int("serverIndex", serverCount).Msg("Creating server connector instance")

		// Create and initialize connector instance
		instance := def.CreateInstance()
		if err := instance.Initialize(config); err != nil {
			log.Error().Err(err).Str("id", id.String()).Str("type", def.ID).Int("serverIndex", serverCount).Msg("Failed to initialize server connector instance")
			continue
		}

		m.instances[id] = connectorInstance{
			connector: instance,
			id:        id,
			config:    config,
			serverID:  &serverID,
		}
		log.Info().Str("id", id.String()).Str("serverID", serverID.String()).Str("type", def.ID).Msg("Initialized server connector")
	}

	if err := serverRows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating server connector rows")
	}
	log.Debug().Int("count", serverCount).Msg("Finished processing server connectors.")

	log.Debug().Msg("Connector initialization finished.")
	return nil
}

// GetConnectorsByType returns all connector instances of a specific type
func (m *ConnectorManager) GetConnectorsByType(connectorType string) []Connector {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var connectors []Connector
	for _, instance := range m.instances {
		if instance.connector.GetDefinition().ID == connectorType {
			connectors = append(connectors, instance.connector)
		}
	}

	return connectors
}

// GetConnectorsByServerAndType returns all connector instances for a specific server and type
func (m *ConnectorManager) GetConnectorsByServerAndType(serverID uuid.UUID, connectorType string) []Connector {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var connectors []Connector
	for _, instance := range m.instances {
		if instance.serverID != nil && *instance.serverID == serverID {
			if instance.connector.GetDefinition().ID == connectorType {
				connectors = append(connectors, instance.connector)
			}
		}
	}

	return connectors
}

// GetConnectorByID returns a connector instance by its ID
func (m *ConnectorManager) GetConnectorByID(id uuid.UUID) (Connector, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, ok := m.instances[id]
	if !ok {
		return nil, false
	}
	return instance.connector, true
}

// GetConnectorsByServer returns all connector instances for a specific server
func (m *ConnectorManager) GetConnectorsByServer(serverID uuid.UUID) []Connector {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var connectors []Connector
	for _, instance := range m.instances {
		// Only include server-specific connectors for this server
		if instance.serverID != nil && *instance.serverID == serverID {
			connectors = append(connectors, instance.connector)
		}
	}

	return connectors
}

// GetConnectorsByServerWithGlobal returns all connector instances for a specific server, including global connectors
func (m *ConnectorManager) GetConnectorsByServerWithGlobal(serverID uuid.UUID) []Connector {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var connectors []Connector
	for _, instance := range m.instances {
		// Include server-specific connectors for this server
		if instance.serverID != nil && *instance.serverID == serverID {
			connectors = append(connectors, instance.connector)
		} else if instance.serverID == nil {
			// Include global connectors
			connectors = append(connectors, instance.connector)
		}
	}

	return connectors
}

// RestartConnector restarts a connector with new configuration
func (m *ConnectorManager) RestartConnector(id uuid.UUID, config map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("connector not found: %s", id)
	}

	// Shutdown existing instance
	if err := instance.connector.Shutdown(); err != nil {
		log.Error().Err(err).Str("id", id.String()).Msg("Failed to shutdown connector")
	}

	// Initialize with new config
	if err := instance.connector.Initialize(config); err != nil {
		return fmt.Errorf("failed to initialize connector with new config: %w", err)
	}

	// Update instance config
	instance.config = config
	m.instances[id] = instance

	log.Info().Str("id", id.String()).Msg("Restarted connector")

	return nil
}

// ShutdownConnector shuts down and removes a connector instance
func (m *ConnectorManager) ShutdownConnector(id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("connector not found: %s", id)
	}

	if err := instance.connector.Shutdown(); err != nil {
		log.Error().Err(err).Str("id", id.String()).Msg("Failed to shutdown connector")
	}

	delete(m.instances, id)

	log.Info().Str("id", id.String()).Msg("Shutdown connector")

	return nil
}

// Shutdown shuts down all connectors
func (m *ConnectorManager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, instance := range m.instances {
		if err := instance.connector.Shutdown(); err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to shutdown connector")
		}
	}

	m.cancel()
}

// SubscribeToEvents subscribes to events from a specific connector
func (m *ConnectorManager) SubscribeToEvents(connectorID string, callback func(event Event) error) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	// First try to parse as UUID for database ID
	var instance connectorInstance
	var ok bool

	if id, err := uuid.Parse(connectorID); err == nil {
		// If it's a valid UUID, look up by instance ID
		instance, ok = m.instances[id]
	} else {
		// If not a UUID, it might be a connector name - log an error
		log.Error().Str("connectorID", connectorID).Msg("Invalid connector ID format - expected UUID")
		return ""
	}

	if !ok {
		log.Error().Str("connectorID", connectorID).Msg("Attempted to subscribe to non-existent connector")
		return ""
	}

	// Get the base connector to access EventEmitter
	base := instance.connector.(ConnectorBaseEventCapable)
	if base.GetEventEmitter() == nil {
		log.Error().Str("connectorID", connectorID).Msg("Connector does not support events")
		return ""
	}

	subscriberID := uuid.New().String()

	// Initialize callbacks map for this connector if needed
	if _, ok := m.subscriberCallbacks[connectorID]; !ok {
		m.subscriberCallbacks[connectorID] = make(map[string]func(data any))
	}

	// Create and store the callback wrapper
	wrapper := func(data any) {
		// For wildcard events, data will be a tuple of (eventType, eventData)
		if tuple, ok := data.([]any); ok && len(tuple) == 2 {
			eventType, ok1 := tuple[0].(string)
			eventData := tuple[1]
			if !ok1 {
				log.Error().Msg("Invalid event type in wildcard event")
				return
			}

			event := Event{
				ConnectorID: connectorID,
				Type:        eventType,
				Data:        eventData,
			}
			if instance.serverID != nil {
				event.ServerID = *instance.serverID
			}
			if err := callback(event); err != nil {
				log.Error().
					Err(err).
					Str("connectorID", connectorID).
					Str("subscriberID", subscriberID).
					Msg("Error handling connector event")
			}
		}
	}
	m.subscriberCallbacks[connectorID][subscriberID] = wrapper

	// Subscribe to all events using "*" as the event type
	base.GetEventEmitter().On("*", wrapper)

	return subscriberID
}

// UnsubscribeFromEvents removes a subscriber
func (m *ConnectorManager) UnsubscribeFromEvents(connectorID string, subscriberID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// First try to parse as UUID for database ID
	var instance connectorInstance
	var ok bool

	if id, err := uuid.Parse(connectorID); err == nil {
		// If it's a valid UUID, look up by instance ID
		instance, ok = m.instances[id]
	} else {
		// If not a UUID, it might be a connector name - log an error
		log.Error().Str("connectorID", connectorID).Msg("Invalid connector ID format - expected UUID")
		return
	}

	if !ok {
		log.Error().Str("connectorID", connectorID).Msg("Attempted to unsubscribe from non-existent connector")
		return
	}

	// Get the base connector to access EventEmitter
	base := instance.connector.(*ConnectorBase)
	if base.EventEmitter == nil {
		log.Error().Str("connectorID", connectorID).Msg("Connector does not support events")
		return
	}

	// Get the stored callback
	if callbacks, ok := m.subscriberCallbacks[connectorID]; ok {
		if callback, ok := callbacks[subscriberID]; ok {
			// Remove the listener using the stored callback
			base.EventEmitter.RemoveListener("*", callback)
			// Clean up the stored callback
			delete(callbacks, subscriberID)
			if len(callbacks) == 0 {
				delete(m.subscriberCallbacks, connectorID)
			}
		}
	}
}

// SubscribeToServerEvents subscribes to events from a specific server's connectors
func (m *ConnectorManager) SubscribeToServerEvents(serverID uuid.UUID, connectorID string, callback func(event Event) error) string {
	// For server events, we can reuse the same subscription mechanism
	// since we already include server ID in the Event struct
	return m.SubscribeToEvents(connectorID, callback)
}

// UnsubscribeFromServerEvents unsubscribes from events from a specific server's connectors
func (m *ConnectorManager) UnsubscribeFromServerEvents(serverID uuid.UUID, connectorID string, subscriberID string) {
	// For server events, we can reuse the same unsubscribe mechanism
	m.UnsubscribeFromEvents(connectorID, subscriberID)
}

// createConnector creates a new connector instance
func (m *ConnectorManager) createConnector(def ConnectorDefinition, id uuid.UUID, config map[string]any, serverID *uuid.UUID) (Connector, error) {
	// Create new instance
	connector := def.CreateInstance()

	// Set ID and config
	base := connector.(*ConnectorBase)
	base.ID = id
	base.Definition = def

	// Initialize the connector
	if err := connector.Initialize(config); err != nil {
		return nil, fmt.Errorf("failed to initialize connector: %w", err)
	}

	return connector, nil
}

// ListRegisteredConnectors returns a list of all registered connector registrars
func (m *ConnectorManager) ListRegisteredConnectors() []ConnectorRegistrar {
	m.mu.RLock()
	defer m.mu.RUnlock()

	registrars := make([]ConnectorRegistrar, 0, len(m.registeredConnectors))
	for _, registrar := range m.registeredConnectors {
		registrars = append(registrars, registrar)
	}
	return registrars
}

// CreateServerConnector creates and enables a server-specific connector with the given type and config.
// It returns the new connector's UUID or an error if creation/initialization fails.
func (m *ConnectorManager) CreateServerConnector(serverID uuid.UUID, connectorType string, config map[string]any) (uuid.UUID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find the connector registrar by type
	registrar, ok := m.registeredConnectors[connectorType]
	if !ok {
		return uuid.Nil, fmt.Errorf("connector type not registered: %s", connectorType)
	}

	// Create a new UUID for the connector
	newID := uuid.New()

	// Get connector definition and create an instance
	def := registrar.Define()
	instance := def.CreateInstance()

	// Initialize the connector with the provided config
	if err := instance.Initialize(config); err != nil {
		return uuid.Nil, fmt.Errorf("failed to initialize server connector: %w", err)
	}

	// Store the new connector instance
	m.instances[newID] = connectorInstance{
		connector: instance,
		id:        newID,
		config:    config,
		serverID:  &serverID,
	}

	// Map the connector instance ID to the server
	m.serverConnectors[serverID] = append(m.serverConnectors[serverID], newID.String())

	log.Info().
		Str("id", newID.String()).
		Str("serverID", serverID.String()).
		Str("type", def.ID).
		Msg("Created server connector")

	return newID, nil
}
