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

// ConnectorInstance represents an instantiated connector
type ConnectorInstance interface {
	// GetID returns the unique identifier for this connector instance
	GetID() uuid.UUID
	// GetType returns the type of connector
	GetType() string
	// GetConfig returns the configuration for this connector
	GetConfig() map[string]interface{}
	// Initialize initializes the connector with its configuration
	Initialize(config map[string]interface{}) error
	// Shutdown gracefully shuts down the connector
	Shutdown() error
}

// ConnectorFactory creates a new connector instance
type ConnectorFactory interface {
	// Create creates a new connector instance
	Create(id uuid.UUID, config map[string]interface{}) (ConnectorInstance, error)
	// GetType returns the type of connector this factory creates
	GetType() string
	// ConfigSchema returns the configuration schema for this connector type
	ConfigSchema() map[string]interface{}
}

// ConnectorManager manages all connectors
type ConnectorManager struct {
	factories    map[string]ConnectorFactory
	instances    map[uuid.UUID]ConnectorInstance
	globalConfig map[string]interface{}
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewConnectorManager creates a new connector manager
func NewConnectorManager(ctx context.Context) *ConnectorManager {
	ctx, cancel := context.WithCancel(ctx)
	return &ConnectorManager{
		factories:    make(map[string]ConnectorFactory),
		instances:    make(map[uuid.UUID]ConnectorInstance),
		globalConfig: make(map[string]interface{}),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// RegisterFactory registers a connector factory
func (m *ConnectorManager) RegisterFactory(factory ConnectorFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()

	connectorType := factory.GetType()
	m.factories[connectorType] = factory
	log.Info().Str("type", connectorType).Msg("Registered connector factory")
}

// ListFactories returns a map of all registered connector factories
func (m *ConnectorManager) ListFactories() map[string]ConnectorFactory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a copy of the factories map to avoid concurrent access issues
	factories := make(map[string]ConnectorFactory)
	for k, v := range m.factories {
		factories[k] = v
	}

	return factories
}

// GetFactory returns a connector factory by its type
func (m *ConnectorManager) GetFactory(connectorType string) (ConnectorFactory, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	factory, ok := m.factories[connectorType]
	return factory, ok
}

// RegisterInstance registers a connector instance
func (m *ConnectorManager) RegisterInstance(id uuid.UUID, instance ConnectorInstance) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.instances[id] = instance
	log.Info().Str("id", id.String()).Str("type", instance.GetType()).Msg("Registered connector instance")
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

		// Get connector type from config
		connectorType, ok := config["type"].(string)
		if !ok {
			log.Error().Str("id", id.String()).Msg("Connector config missing type field")
			continue
		}

		// Find factory for this connector type
		factory, ok := m.factories[connectorType]
		if !ok {
			log.Error().Str("type", connectorType).Msg("No factory registered for connector type")
			continue
		}

		// Create and initialize connector instance
		instance, err := factory.Create(id, config)
		if err != nil {
			log.Error().Err(err).Str("id", id.String()).Str("type", connectorType).Msg("Failed to create connector instance")
			continue
		}

		m.instances[id] = instance
		log.Info().Str("id", id.String()).Str("type", connectorType).Msg("Initialized global connector")
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

		// Get connector type from config
		connectorType, ok := config["type"].(string)
		if !ok {
			log.Error().Str("id", id.String()).Msg("Server connector config missing type field")
			continue
		}

		// Find factory for this connector type
		factory, ok := m.factories[connectorType]
		if !ok {
			log.Error().Str("type", connectorType).Msg("No factory registered for connector type")
			continue
		}

		// Create and initialize connector instance
		instance, err := factory.Create(id, config)
		if err != nil {
			log.Error().Err(err).Str("id", id.String()).Str("type", connectorType).Msg("Failed to create server connector instance")
			continue
		}

		m.instances[id] = instance
		log.Info().Str("id", id.String()).Str("serverID", serverID.String()).Str("type", connectorType).Msg("Initialized server connector")
	}

	if err := serverRows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating server connector rows")
	}

	return nil
}

// GetConnectorsByType returns all connector instances of a specific type
func (m *ConnectorManager) GetConnectorsByType(connectorType string) ([]ConnectorInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var connectors []ConnectorInstance
	for _, instance := range m.instances {
		if instance.GetType() == connectorType {
			connectors = append(connectors, instance)
		}
	}

	return connectors, nil
}

// GetConnectorsByServerAndType returns all connector instances for a specific server and type
func (m *ConnectorManager) GetConnectorsByServerAndType(serverID uuid.UUID, connectorType string) ([]ConnectorInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var connectors []ConnectorInstance
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
func (m *ConnectorManager) GetConnectorByID(id uuid.UUID) (ConnectorInstance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, ok := m.instances[id]
	return instance, ok
}

// GetConnectorsByServer returns all connector instances for a specific server
func (m *ConnectorManager) GetConnectorsByServer(serverID uuid.UUID) []ConnectorInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var connectors []ConnectorInstance
	for _, instance := range m.instances {
		config := instance.GetConfig()
		if serverIDStr, ok := config["server_id"].(string); ok {
			if configServerID, err := uuid.Parse(serverIDStr); err == nil && configServerID == serverID {
				connectors = append(connectors, instance)
			}
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

	// Preserve connector type
	connectorType := instance.GetType()
	config["type"] = connectorType

	// Get the factory for this connector type
	factory, ok := m.factories[connectorType]
	if !ok {
		return fmt.Errorf("no factory registered for connector type: %s", connectorType)
	}

	// Create a new instance with the updated config
	newInstance, err := factory.Create(id, config)
	if err != nil {
		return fmt.Errorf("failed to create new connector instance: %w", err)
	}

	// Store the new instance
	m.instances[id] = newInstance
	log.Info().Str("id", id.String()).Str("type", connectorType).Msg("Restarted connector")

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
