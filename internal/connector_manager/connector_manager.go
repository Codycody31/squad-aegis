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
	registeredConnectors map[string]ConnectorRegistrar
	instances            map[uuid.UUID]Connector
	globalConfig         map[string]interface{}
	mu                   sync.RWMutex
	ctx                  context.Context
	cancel               context.CancelFunc
}

// NewConnectorManager creates a new connector manager
func NewConnectorManager(ctx context.Context) *ConnectorManager {
	ctx, cancel := context.WithCancel(ctx)
	return &ConnectorManager{
		registeredConnectors: make(map[string]ConnectorRegistrar),
		instances:            make(map[uuid.UUID]Connector),
		globalConfig:         make(map[string]interface{}),
		ctx:                  ctx,
		cancel:               cancel,
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

	// Query for global connectors
	globalRows, err := db.QueryContext(ctx, `
		SELECT id, name, config
		FROM connectors
	`)
	if err != nil {
		return fmt.Errorf("failed to query connectors: %w", err)
	}
	defer globalRows.Close()

	// Initialize global connectors
	for globalRows.Next() {
		var id uuid.UUID
		var name string
		var configJSON []byte

		if err := globalRows.Scan(&id, &name, &configJSON); err != nil {
			log.Error().Err(err).Msg("Failed to scan connector row")
			continue
		}

		var config map[string]interface{}
		if err := json.Unmarshal(configJSON, &config); err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to unmarshal connector config")
			continue
		}

		// Find registrar for this connector type
		registrar, ok := m.registeredConnectors[name]
		if !ok {
			log.Error().Str("name", name).Msg("No registrar found for connector type")
			continue
		}

		// Get connector definition
		def := registrar.Define()

		// Create and initialize connector instance
		instance, err := def.CreateInstance(id, config)
		if err != nil {
			log.Error().Err(err).Str("id", id.String()).Str("type", def.ID).Msg("Failed to create connector instance")
			continue
		}

		m.instances[id] = instance
		log.Info().Str("id", id.String()).Str("type", def.ID).Msg("Initialized global connector")
	}

	if err := globalRows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating connector rows")
	}

	// Query for server-specific connectors
	serverRows, err := db.QueryContext(ctx, `
		SELECT id, server_id, name, config
		FROM server_connectors
	`)
	if err != nil {
		return fmt.Errorf("failed to query server connectors: %w", err)
	}
	defer serverRows.Close()

	// Initialize server-specific connectors
	for serverRows.Next() {
		var id uuid.UUID
		var serverID uuid.UUID
		var name string
		var configJSON []byte

		if err := serverRows.Scan(&id, &serverID, &name, &configJSON); err != nil {
			log.Error().Err(err).Msg("Failed to scan server connector row")
			continue
		}

		var config map[string]interface{}
		if err := json.Unmarshal(configJSON, &config); err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to unmarshal server connector config")
			continue
		}

		// Add server ID to config
		config["server_id"] = serverID.String()

		// Find registrar for this connector type
		registrar, ok := m.registeredConnectors[name]
		if !ok {
			log.Error().Str("name", name).Msg("No registrar found for connector type")
			continue
		}

		// Get connector definition
		def := registrar.Define()

		// Create and initialize connector instance
		instance, err := def.CreateInstance(id, config)
		if err != nil {
			log.Error().Err(err).Str("id", id.String()).Str("type", def.ID).Msg("Failed to create server connector instance")
			continue
		}

		m.instances[id] = instance
		log.Info().Str("id", id.String()).Str("serverID", serverID.String()).Str("type", def.ID).Msg("Initialized server connector")
	}

	if err := serverRows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating server connector rows")
	}

	return nil
}

// GetConnectorsByType returns all connector instances of a specific type
func (m *ConnectorManager) GetConnectorsByType(connectorType string) ([]Connector, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var connectors []Connector
	for _, instance := range m.instances {
		if instance.GetType() == connectorType {
			connectors = append(connectors, instance)
		}
	}

	return connectors, nil
}

// GetConnectorsByServerAndType returns all connector instances for a specific server and type
func (m *ConnectorManager) GetConnectorsByServerAndType(serverID uuid.UUID, connectorType string) ([]Connector, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var connectors []Connector
	for _, instance := range m.instances {
		config := instance.GetConfig()
		if serverIDStr, ok := config["server_id"].(string); ok {
			if configServerID, err := uuid.Parse(serverIDStr); err == nil && configServerID == serverID {
				if instance.GetType() == connectorType {
					connectors = append(connectors, instance)
				}
			}
		}
	}

	return connectors, nil
}

// GetConnectorByID returns a connector instance by its ID
func (m *ConnectorManager) GetConnectorByID(id uuid.UUID) (Connector, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, ok := m.instances[id]
	return instance, ok
}

// GetConnectorsByServer returns all connector instances for a specific server
func (m *ConnectorManager) GetConnectorsByServer(serverID uuid.UUID) []Connector {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var connectors []Connector
	for _, instance := range m.instances {
		config := instance.GetConfig()
		// Include server-specific connectors
		if serverIDStr, ok := config["server_id"].(string); ok {
			if configServerID, err := uuid.Parse(serverIDStr); err == nil && configServerID == serverID {
				connectors = append(connectors, instance)
			}
		} else {
			// Include global connectors (those without a server_id)
			connectors = append(connectors, instance)
		}
	}

	return connectors
}

// RestartConnector restarts a connector with new configuration
func (m *ConnectorManager) RestartConnector(id uuid.UUID, config map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get the existing instance
	instance, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("connector not found: %s", id)
	}

	// Shutdown the existing instance
	if err := instance.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown connector: %w", err)
	}

	// Get the connector definition
	def := instance.GetDefinition()

	// Find registrar for this connector type
	registrarName := def.ID
	registrar, ok := m.registeredConnectors[registrarName]
	if !ok {
		return fmt.Errorf("no registrar found for connector type: %s", registrarName)
	}

	// Create a new instance with the updated config
	newDef := registrar.Define()
	newInstance, err := newDef.CreateInstance(id, config)
	if err != nil {
		return fmt.Errorf("failed to create new connector instance: %w", err)
	}

	// Store the new instance
	m.instances[id] = newInstance
	log.Info().Str("id", id.String()).Str("type", def.ID).Msg("Restarted connector")

	return nil
}

// ShutdownConnector shuts down and removes a connector
func (m *ConnectorManager) ShutdownConnector(id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get the existing instance
	instance, ok := m.instances[id]
	if !ok {
		// Already gone, nothing to do
		return nil
	}

	// Shutdown the instance
	if err := instance.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown connector: %w", err)
	}

	// Remove the instance
	delete(m.instances, id)
	log.Info().Str("id", id.String()).Str("type", instance.GetType()).Msg("Removed connector")

	return nil
}

// Shutdown shuts down all connectors
func (m *ConnectorManager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel context
	m.cancel()

	// Shutdown all connector instances
	for id, instance := range m.instances {
		if err := instance.Shutdown(); err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Error shutting down connector")
		}
	}

	log.Info().Msg("All connectors shut down")
}
