package plugin_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Database operations for plugin manager

func (pm *PluginManager) loadPluginsFromDatabase() error {
	query := `
		SELECT id, server_id, plugin_id, notes, config, enabled, created_at, updated_at
		FROM plugin_instances
		ORDER BY created_at
	`

	rows, err := pm.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query plugin instances: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var instance PluginInstance
		var configJSON string

		err := rows.Scan(
			&instance.ID,
			&instance.ServerID,
			&instance.PluginID,
			&instance.Notes,
			&configJSON,
			&instance.Enabled,
			&instance.CreatedAt,
			&instance.UpdatedAt,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan plugin instance row")
			continue
		}

		// Parse config JSON
		if err := json.Unmarshal([]byte(configJSON), &instance.Config); err != nil {
			log.Error().
				Str("instanceID", instance.ID.String()).
				Err(err).
				Msg("Failed to parse plugin instance config")
			continue
		}

		// Create plugin instance
		plugin, err := pm.registry.CreatePluginInstance(instance.PluginID)
		if err != nil {
			log.Error().
				Str("instanceID", instance.ID.String()).
				Str("pluginID", instance.PluginID).
				Err(err).
				Msg("Failed to create plugin instance")
			continue
		}

		// Set up instance
		ctx, cancel := context.WithCancel(pm.ctx)
		instance.Plugin = plugin
		instance.Context = ctx
		instance.Cancel = cancel
		instance.Status = PluginStatusStopped

		// Initialize server plugins map if needed
		if pm.plugins[instance.ServerID] == nil {
			pm.plugins[instance.ServerID] = make(map[uuid.UUID]*PluginInstance)
		}

		// Store instance
		pm.plugins[instance.ServerID][instance.ID] = &instance

		// Only initialize plugin if enabled
		if instance.Enabled {
			// Initialize plugin
			if err := pm.initializePluginInstance(&instance); err != nil {
				log.Error().
					Str("instanceID", instance.ID.String()).
					Str("pluginID", instance.PluginID).
					Err(err).
					Msg("Failed to initialize plugin instance")

				instance.Status = PluginStatusError
				instance.LastError = err.Error()
				continue
			}
		} else {
			// Set status for disabled plugins
			instance.Status = PluginStatusDisabled
		}

		log.Info().
			Str("serverID", instance.ServerID.String()).
			Str("instanceID", instance.ID.String()).
			Str("pluginID", instance.PluginID).
			Str("notes", instance.Notes).
			Msg("Loaded plugin instance from database")
	}

	return nil
}

func (pm *PluginManager) savePluginInstanceToDatabase(instance *PluginInstance) error {
	configJSON, err := json.Marshal(instance.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		INSERT INTO plugin_instances (id, server_id, plugin_id, notes, config, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = pm.db.Exec(query,
		instance.ID,
		instance.ServerID,
		instance.PluginID,
		instance.Notes,
		string(configJSON),
		instance.Enabled,
		instance.CreatedAt,
		instance.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert plugin instance: %w", err)
	}

	return nil
}

func (pm *PluginManager) updatePluginInstanceInDatabase(instance *PluginInstance) error {
	configJSON, err := json.Marshal(instance.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		UPDATE plugin_instances
		SET notes = $2, config = $3, enabled = $4, updated_at = $5
		WHERE id = $1
	`

	_, err = pm.db.Exec(query,
		instance.ID,
		instance.Notes,
		string(configJSON),
		instance.Enabled,
		instance.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update plugin instance: %w", err)
	}

	return nil
}

func (pm *PluginManager) deletePluginInstanceFromDatabase(instanceID uuid.UUID) error {
	// Delete plugin data first
	_, err := pm.db.Exec("DELETE FROM plugin_data WHERE plugin_instance_id = $1", instanceID)
	if err != nil {
		log.Error().Str("instanceID", instanceID.String()).Err(err).Msg("Failed to delete plugin data")
	}

	// Delete plugin instance
	query := `DELETE FROM plugin_instances WHERE id = $1`
	_, err = pm.db.Exec(query, instanceID)
	if err != nil {
		return fmt.Errorf("failed to delete plugin instance: %w", err)
	}

	return nil
}

// Connector database operations

func (pm *PluginManager) startConnectors() error {
	query := `
		SELECT id, config, enabled, created_at, updated_at
		FROM connectors
		WHERE enabled = true
		ORDER BY created_at
	`

	rows, err := pm.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query connectors: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var instance ConnectorInstance
		var configJSON string

		err := rows.Scan(
			&instance.ID,
			&configJSON,
			&instance.Enabled,
			&instance.CreatedAt,
			&instance.UpdatedAt,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan connector row")
			continue
		}

		// Parse config JSON
		if err := json.Unmarshal([]byte(configJSON), &instance.Config); err != nil {
			log.Error().
				Str("connectorID", instance.ID).
				Err(err).
				Msg("Failed to parse connector config")
			continue
		}

		// Create connector instance
		connector, err := pm.connectorRegistry.CreateConnectorInstance(instance.ID)
		if err != nil {
			log.Error().
				Str("connectorID", instance.ID).
				Err(err).
				Msg("Failed to create connector instance")
			continue
		}

		// Set up instance
		ctx, cancel := context.WithCancel(pm.ctx)
		instance.Connector = connector
		instance.Context = ctx
		instance.Cancel = cancel
		instance.Status = ConnectorStatusStopped

		// Store instance
		pm.connectors[instance.ID] = &instance

		// Initialize and start connector
		if err := pm.initializeConnectorInstance(&instance); err != nil {
			log.Error().
				Str("connectorID", instance.ID).
				Err(err).
				Msg("Failed to initialize connector instance")

			instance.Status = ConnectorStatusError
			instance.LastError = err.Error()
			continue
		}

		log.Info().
			Str("connectorID", instance.ID).
			Msg("Started connector instance")
	}

	return nil
}

func (pm *PluginManager) initializeConnectorInstance(instance *ConnectorInstance) error {
	instance.Status = ConnectorStatusStarting

	// Initialize connector
	if err := instance.Connector.Initialize(instance.Config); err != nil {
		instance.Status = ConnectorStatusError
		instance.LastError = err.Error()
		return fmt.Errorf("failed to initialize connector: %w", err)
	}

	// Start connector
	if err := instance.Connector.Start(instance.Context); err != nil {
		instance.Status = ConnectorStatusError
		instance.LastError = err.Error()
		return fmt.Errorf("failed to start connector: %w", err)
	}

	instance.Status = ConnectorStatusRunning
	instance.LastError = ""
	return nil
}

func (pm *PluginManager) stopConnectorInstance(instance *ConnectorInstance) error {
	instance.Status = ConnectorStatusStopping

	// Cancel context
	if instance.Cancel != nil {
		instance.Cancel()
	}

	// Stop connector with 30-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stopChan := make(chan error, 1)

	go func() {
		stopChan <- instance.Connector.Stop()
	}()

	select {
	case err := <-stopChan:
		if err != nil {
			instance.Status = ConnectorStatusError
			instance.LastError = err.Error()
			return fmt.Errorf("failed to stop connector: %w", err)
		}
		instance.Status = ConnectorStatusStopped
		return nil
	case <-ctx.Done():
		log.Warn().
			Str("connectorID", instance.ID).
			Msg("Connector shutdown timed out after 30 seconds, forcefully killing it")
		instance.Status = ConnectorStatusStopped
		instance.LastError = "Connector shutdown timed out after 30 seconds"
		return nil
	}
}

func (pm *PluginManager) saveConnectorToDatabase(instance *ConnectorInstance) error {
	configJSON, err := json.Marshal(instance.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		INSERT INTO connectors (id, config, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			config = EXCLUDED.config,
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at
	`

	_, err = pm.db.Exec(query,
		instance.ID,
		string(configJSON),
		instance.Enabled,
		instance.CreatedAt,
		instance.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save connector: %w", err)
	}

	return nil
}

// CreateConnectorInstance creates and starts a new connector instance
func (pm *PluginManager) CreateConnectorInstance(connectorID string, config map[string]interface{}) (*ConnectorInstance, error) {
	pm.connectorMu.Lock()
	defer pm.connectorMu.Unlock()

	// Check if connector already exists
	if _, exists := pm.connectors[connectorID]; exists {
		return nil, fmt.Errorf("connector %s already exists", connectorID)
	}

	// Get connector definition to validate config
	definition, err := pm.connectorRegistry.GetConnector(connectorID)
	if err != nil {
		return nil, fmt.Errorf("connector definition not found: %w", err)
	}

	// Validate config for creation (ensures sensitive required fields are provided)
	if err := definition.ConfigSchema.ValidateForCreation(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Fill defaults
	config = definition.ConfigSchema.FillDefaults(config)

	// Create connector instance
	connector, err := pm.connectorRegistry.CreateConnectorInstance(connectorID)
	if err != nil {
		return nil, fmt.Errorf("failed to create connector instance: %w", err)
	}

	// Create instance record
	ctx, cancel := context.WithCancel(pm.ctx)

	instance := &ConnectorInstance{
		ID:        connectorID,
		Config:    config,
		Status:    ConnectorStatusStopped,
		Enabled:   true,
		Connector: connector,
		Context:   ctx,
		Cancel:    cancel,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Store instance
	pm.connectors[connectorID] = instance

	// Save to database
	if err := pm.saveConnectorToDatabase(instance); err != nil {
		delete(pm.connectors, connectorID)
		cancel()
		return nil, fmt.Errorf("failed to save connector to database: %w", err)
	}

	// Initialize and start connector
	if err := pm.initializeConnectorInstance(instance); err != nil {
		delete(pm.connectors, connectorID)
		cancel()
		return nil, fmt.Errorf("failed to initialize connector instance: %w", err)
	}

	log.Info().
		Str("connectorID", connectorID).
		Msg("Created connector instance")

	return instance, nil
}

// UpdateConnectorConfig updates a connector's configuration
func (pm *PluginManager) UpdateConnectorConfig(connectorID string, config map[string]interface{}) error {
	pm.connectorMu.Lock()
	defer pm.connectorMu.Unlock()

	instance, exists := pm.connectors[connectorID]
	if !exists {
		return fmt.Errorf("connector %s not found", connectorID)
	}

	// Get connector definition to validate config
	definition, err := pm.connectorRegistry.GetConnector(connectorID)
	if err != nil {
		return fmt.Errorf("connector definition not found: %w", err)
	}

	// Merge new config with existing, handling sensitive fields properly
	mergedConfig := definition.ConfigSchema.MergeConfigUpdates(instance.Config, config)

	// Validate the merged config
	if err := definition.ConfigSchema.Validate(mergedConfig); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Update connector config
	if err := instance.Connector.UpdateConfig(mergedConfig); err != nil {
		return fmt.Errorf("failed to update connector config: %w", err)
	}

	// Restart the connector if needed (some connectors stop themselves during UpdateConfig)
	if instance.Connector.GetStatus() != ConnectorStatusRunning {
		if err := instance.Connector.Start(instance.Context); err != nil {
			return fmt.Errorf("failed to restart connector after config update: %w", err)
		}
	}

	// Update instance record
	instance.Config = mergedConfig
	instance.UpdatedAt = time.Now()

	// Save to database
	if err := pm.saveConnectorToDatabase(instance); err != nil {
		return fmt.Errorf("failed to update connector in database: %w", err)
	}

	// Restart dependent plugins after connector update
	if err := pm.restartDependentPlugins(connectorID); err != nil {
		log.Error().
			Str("connectorID", connectorID).
			Err(err).
			Msg("Failed to restart dependent plugins after connector update")
		// Don't return error as the connector update was successful
	}

	return nil
}

// DeleteConnectorInstance removes and stops a connector instance
func (pm *PluginManager) DeleteConnectorInstance(connectorID string) error {
	pm.connectorMu.Lock()
	defer pm.connectorMu.Unlock()

	instance, exists := pm.connectors[connectorID]
	if !exists {
		return fmt.Errorf("connector %s not found", connectorID)
	}

	// Stop connector
	if err := pm.stopConnectorInstance(instance); err != nil {
		log.Error().
			Str("connectorID", connectorID).
			Err(err).
			Msg("Failed to stop connector instance during deletion")
	}

	// Remove from database
	_, err := pm.db.Exec("DELETE FROM connectors WHERE id = $1", connectorID)
	if err != nil {
		log.Error().
			Str("connectorID", connectorID).
			Err(err).
			Msg("Failed to delete connector from database")
	}

	// Remove from memory
	delete(pm.connectors, connectorID)

	log.Info().
		Str("connectorID", connectorID).
		Msg("Deleted connector instance")

	return nil
}
