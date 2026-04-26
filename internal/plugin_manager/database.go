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
		SELECT id, server_id, plugin_id, notes, config, enabled, log_level, created_at, updated_at
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
			&instance.LogLevel,
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

		if err := pm.hydratePluginInstanceFromDatabase(&instance); err != nil {
			log.Error().
				Str("instanceID", instance.ID.String()).
				Str("pluginID", instance.PluginID).
				Err(err).
				Msg("Failed to hydrate plugin instance from database")
			continue
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate plugin instances: %w", err)
	}

	return nil
}

func (pm *PluginManager) hydratePluginInstanceFromDatabase(instance *PluginInstance) error {
	definition, err := pm.registry.GetPlugin(instance.PluginID)
	if err != nil {
		pm.markPluginInstanceUnavailable(instance, err)
		pm.storeLoadedPluginInstance(instance)

		log.Warn().
			Str("serverID", instance.ServerID.String()).
			Str("instanceID", instance.ID.String()).
			Str("pluginID", instance.PluginID).
			Msg("Loaded persisted plugin instance without an available plugin definition")
		return nil
	}

	pm.applyPluginDefinitionMetadata(instance, pm.enrichPluginDefinition(*definition))

	if err := pm.ensurePluginInstanceRuntime(instance); err != nil {
		pm.markPluginInstanceUnavailable(instance, err)
		pm.storeLoadedPluginInstance(instance)

		log.Warn().
			Str("serverID", instance.ServerID.String()).
			Str("instanceID", instance.ID.String()).
			Str("pluginID", instance.PluginID).
			Err(err).
			Msg("Loaded persisted plugin instance without an available runtime")
		return nil
	}

	instance.setStatus(PluginStatusStopped)
	pm.storeLoadedPluginInstance(instance)

	if instance.Enabled {
		if err := pm.initializePluginInstance(instance); err != nil {
			log.Error().
				Str("instanceID", instance.ID.String()).
				Str("pluginID", instance.PluginID).
				Err(err).
				Msg("Failed to initialize plugin instance")

			instance.setError(PluginStatusError, err.Error())
			return nil
		}
	} else {
		instance.setStatus(PluginStatusDisabled)
	}

	log.Info().
		Str("serverID", instance.ServerID.String()).
		Str("instanceID", instance.ID.String()).
		Str("pluginID", instance.PluginID).
		Str("notes", instance.Notes).
		Msg("Loaded plugin instance from database")

	return nil
}

func (pm *PluginManager) applyPluginDefinitionMetadata(instance *PluginInstance, definition PluginDefinition) {
	instance.PluginName = definition.Name
	instance.Source = definition.Source
	instance.Official = definition.Official
	instance.Distribution = definition.Distribution
	instance.InstallState = definition.InstallState
	instance.MinHostAPIVersion = definition.MinHostAPIVersion
}

func (pm *PluginManager) markPluginInstanceUnavailable(instance *PluginInstance, cause error) {
	pm.ensurePluginInstanceContext(instance)

	if pkg := pm.getNativePackage(instance.PluginID); pkg != nil {
		if pkg.Name != "" {
			instance.PluginName = pkg.Name
		}
		instance.Source = pkg.Source
		instance.Official = pkg.Official
		instance.Distribution = pkg.Distribution
		instance.InstallState = pkg.InstallState
		instance.MinHostAPIVersion = pkg.MinHostAPIVersion
		if pkg.LastError != "" {
			instance.mu.Lock()
			instance.LastError = pkg.LastError
			instance.mu.Unlock()
		}
	}

	if instance.PluginName == "" {
		instance.PluginName = instance.PluginID
	}
	if instance.LastError == "" {
		instance.mu.Lock()
		instance.LastError = fmt.Sprintf("plugin definition unavailable: %v", cause)
		instance.mu.Unlock()
	}
	if instance.Enabled {
		instance.setStatus(PluginStatusError)
		return
	}

	instance.setStatus(PluginStatusDisabled)
}

func (pm *PluginManager) ensurePluginInstanceContext(instance *PluginInstance) {
	if instance.Context != nil && instance.Cancel != nil {
		return
	}

	baseCtx := pm.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	ctx, cancel := context.WithCancel(baseCtx)
	instance.Context = ctx
	instance.Cancel = cancel
}

func (pm *PluginManager) storeLoadedPluginInstance(instance *PluginInstance) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.plugins[instance.ServerID] == nil {
		pm.plugins[instance.ServerID] = make(map[uuid.UUID]*PluginInstance)
	}
	pm.plugins[instance.ServerID][instance.ID] = instance
}

func (pm *PluginManager) savePluginInstanceToDatabase(instance *PluginInstance) error {
	configJSON, err := json.Marshal(instance.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		INSERT INTO plugin_instances (id, server_id, plugin_id, notes, config, enabled, log_level, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err = pm.db.Exec(query,
		instance.ID,
		instance.ServerID,
		instance.PluginID,
		instance.Notes,
		string(configJSON),
		instance.Enabled,
		instance.LogLevel,
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
		SET notes = $2, config = $3, enabled = $4, log_level = $5, updated_at = $6
		WHERE id = $1
	`

	_, err = pm.db.Exec(query,
		instance.ID,
		instance.Notes,
		string(configJSON),
		instance.Enabled,
		instance.LogLevel,
		instance.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update plugin instance: %w", err)
	}

	return nil
}

func (pm *PluginManager) deletePluginInstanceFromDatabase(instanceID uuid.UUID) error {
	tx, err := pm.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM plugin_data WHERE plugin_instance_id = $1", instanceID); err != nil {
		return fmt.Errorf("failed to delete plugin data: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM plugin_instances WHERE id = $1", instanceID); err != nil {
		return fmt.Errorf("failed to delete plugin instance: %w", err)
	}
	return tx.Commit()
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
		pm.connectorMu.Lock()
		pm.connectors[instance.ID] = &instance
		pm.connectorMu.Unlock()

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

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating connector rows: %w", err)
	}

	return nil
}

func (pm *PluginManager) initializeConnectorInstance(instance *ConnectorInstance) error {
	instance.Status = ConnectorStatusStarting

	// Initialize connector (panic-safe so a crashing native connector cannot
	// take down the manager).
	if err := safePluginCall(instance.ID, "Connector.Initialize", func() error {
		return instance.Connector.Initialize(instance.Config)
	}); err != nil {
		instance.Status = ConnectorStatusError
		instance.LastError = err.Error()
		return fmt.Errorf("failed to initialize connector: %w", err)
	}

	// Wire health reporting for subprocess-isolated connectors so a
	// crashing subprocess flips the instance to the error state.
	pm.attachConnectorUnexpectedExitReporter(instance)

	// Start connector
	if err := safePluginCall(instance.ID, "Connector.Start", func() error {
		return instance.Connector.Start(instance.Context)
	}); err != nil {
		// Start failed after a successful Initialize. Tear down the
		// connector so any subprocess/goroutine/RPC listener spawned by
		// Initialize is released; otherwise the instance is wedged and the
		// next Enable hits "connector subprocess already initialized".
		if stopErr := safePluginCall(instance.ID, "Connector.Stop", instance.Connector.Stop); stopErr != nil {
			log.Warn().
				Err(stopErr).
				Str("connector_id", instance.ID).
				Msg("Failed to stop connector after Start error; subprocess may leak")
		}
		instance.Status = ConnectorStatusError
		instance.LastError = err.Error()
		return fmt.Errorf("failed to start connector: %w", err)
	}

	instance.Status = ConnectorStatusRunning
	instance.LastError = ""
	return nil
}

// attachConnectorUnexpectedExitReporter wires the instance-level error state
// into the connector shim's health watcher so a crashing subprocess flips
// the connector to the error state.
//
// The callback must not acquire pm.connectorMu: if it did, it would deadlock
// when an admin caller (Delete/Disable) holds pm.connectorMu while waiting
// on Connector.Stop(), which itself waits on the watcher goroutine to exit.
// The captured `instance` pointer stays valid for the lifetime of the
// closure, and instance.setError is synchronized via instance.mu.
func (pm *PluginManager) attachConnectorUnexpectedExitReporter(instance *ConnectorInstance) {
	reporter, ok := instance.Connector.(unexpectedExitReporter)
	if !ok {
		return
	}
	connectorID := instance.ID
	reporter.OnUnexpectedExit(func(err error) {
		instance.setError(err)
		log.Warn().
			Err(err).
			Str("connector_id", connectorID).
			Msg("Connector subprocess exited unexpectedly; instance marked as errored")
	})
}

// killableConnector is implemented by subprocess-isolated connectors that can
// SIGKILL their backing process when a graceful Stop() hangs. In-process
// connectors do not implement this and are skipped.
type killableConnector interface {
	Kill() error
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
		if killer, ok := instance.Connector.(killableConnector); ok {
			if killErr := killer.Kill(); killErr != nil {
				log.Warn().
					Str("connectorID", instance.ID).
					Err(killErr).
					Msg("Failed to SIGKILL connector subprocess after stop timeout")
			}
		}
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
	definition, err := pm.connectorRegistry.GetConnector(connectorID)
	if err != nil {
		return nil, fmt.Errorf("connector definition not found: %w", err)
	}

	storageKey := definition.ConnectorInstanceStorageKey()

	pm.connectorMu.Lock()
	defer pm.connectorMu.Unlock()

	// Check if connector already exists
	if _, exists := pm.connectors[storageKey]; exists {
		return nil, fmt.Errorf("connector %s already exists", storageKey)
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
		ID:        storageKey,
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
	pm.connectors[storageKey] = instance

	// Save to database
	if err := pm.saveConnectorToDatabase(instance); err != nil {
		delete(pm.connectors, storageKey)
		cancel()
		return nil, fmt.Errorf("failed to save connector to database: %w", err)
	}

	// Initialize and start connector
	if err := pm.initializeConnectorInstance(instance); err != nil {
		delete(pm.connectors, storageKey)
		cancel()
		return nil, fmt.Errorf("failed to initialize connector instance: %w", err)
	}

	log.Info().
		Str("connectorID", storageKey).
		Msg("Created connector instance")

	return instance, nil
}

// UpdateConnectorConfig updates a connector's configuration
func (pm *PluginManager) UpdateConnectorConfig(connectorID string, config map[string]interface{}) error {
	storageKey, ok := pm.ResolveConnectorInstanceKey(connectorID)
	if !ok {
		return fmt.Errorf("connector %s not found", connectorID)
	}

	// Snapshot the instance and validate under the lock, then release before
	// the subprocess RPC so concurrent connector operations are not stalled
	// for seconds at a time.
	pm.connectorMu.Lock()
	instance, exists := pm.connectors[storageKey]
	if !exists {
		pm.connectorMu.Unlock()
		return fmt.Errorf("connector %s not found", connectorID)
	}

	definition, err := pm.connectorRegistry.GetConnector(connectorID)
	if err != nil {
		pm.connectorMu.Unlock()
		return fmt.Errorf("connector definition not found: %w", err)
	}

	mergedConfig := definition.ConfigSchema.MergeConfigUpdates(instance.Config, config)
	if err := definition.ConfigSchema.Validate(mergedConfig); err != nil {
		pm.connectorMu.Unlock()
		return fmt.Errorf("config validation failed: %w", err)
	}

	connector := instance.Connector
	instanceCtx := instance.Context
	pm.connectorMu.Unlock()

	if err := connector.UpdateConfig(mergedConfig); err != nil {
		return fmt.Errorf("failed to update connector config: %w", err)
	}

	if connector.GetStatus() != ConnectorStatusRunning {
		if err := connector.Start(instanceCtx); err != nil {
			return fmt.Errorf("failed to restart connector after config update: %w", err)
		}
	}

	// Re-acquire the lock to commit the new config. If the instance was
	// deleted or replaced while we were doing RPC, discard our changes.
	pm.connectorMu.Lock()
	current, stillExists := pm.connectors[storageKey]
	if !stillExists || current != instance {
		pm.connectorMu.Unlock()
		return fmt.Errorf("connector %s was modified concurrently; config update discarded", connectorID)
	}
	instance.Config = mergedConfig
	instance.UpdatedAt = time.Now()
	if err := pm.saveConnectorToDatabase(instance); err != nil {
		pm.connectorMu.Unlock()
		return fmt.Errorf("failed to update connector in database: %w", err)
	}
	pm.connectorMu.Unlock()

	if err := pm.restartDependentPlugins(storageKey); err != nil {
		log.Error().
			Str("connectorID", storageKey).
			Err(err).
			Msg("Failed to restart dependent plugins after connector update")
	}

	return nil
}

// DeleteConnectorInstance removes and stops a connector instance
func (pm *PluginManager) DeleteConnectorInstance(connectorID string) error {
	storageKey, ok := pm.ResolveConnectorInstanceKey(connectorID)
	if !ok {
		return fmt.Errorf("connector %s not found", connectorID)
	}

	pm.connectorMu.Lock()
	defer pm.connectorMu.Unlock()

	instance, exists := pm.connectors[storageKey]
	if !exists {
		return fmt.Errorf("connector %s not found", connectorID)
	}

	// Stop connector
	if err := pm.stopConnectorInstance(instance); err != nil {
		log.Error().
			Str("connectorID", storageKey).
			Err(err).
			Msg("Failed to stop connector instance during deletion")
	}

	// Remove from database
	_, err := pm.db.Exec("DELETE FROM connectors WHERE id = $1", storageKey)
	if err != nil {
		log.Error().
			Str("connectorID", storageKey).
			Err(err).
			Msg("Failed to delete connector from database")
		return fmt.Errorf("failed to delete connector from database: %w", err)
	}

	// Remove from memory
	delete(pm.connectors, storageKey)

	log.Info().
		Str("connectorID", storageKey).
		Msg("Deleted connector instance")

	return nil
}
