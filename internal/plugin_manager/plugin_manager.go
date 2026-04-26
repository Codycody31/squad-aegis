package plugin_manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/clickhouse"
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
	db               *sql.DB
	eventManager     *event_manager.EventManager
	rconManager      *rcon_manager.RconManager
	clickhouseClient *clickhouse.Client

	// Connector management
	connectors        map[string]*ConnectorInstance
	connectorRegistry ConnectorRegistry
	connectorMu       sync.RWMutex

	// Native plugin packages
	nativePackages      map[string]*InstalledPluginPackage
	loadedNativePlugins map[string]string
	nativeMu            sync.RWMutex

	// Native connector packages
	nativeConnectorPackages map[string]*InstalledConnectorPackage
	loadedNativeConnectors  map[string]string

	// installMu serializes the full native plugin/connector install and
	// delete flow. nativeMu / connectorMu alone are insufficient because
	// the install flow spans multiple separate critical sections.
	installMu sync.Mutex

	// Event subscription
	eventSubscriber *event_manager.EventSubscriber

	// Ban sync callback (set by server after construction)
	banSyncFunc func(ctx context.Context, serverID uuid.UUID) error
}

// NewPluginManager creates a new plugin manager
func NewPluginManager(ctx context.Context, db *sql.DB, eventManager *event_manager.EventManager, rconManager *rcon_manager.RconManager, clickhouseClient *clickhouse.Client) *PluginManager {
	ctx, cancel := context.WithCancel(ctx)

	pm := &PluginManager{
		plugins:                 make(map[uuid.UUID]map[uuid.UUID]*PluginInstance),
		registry:                NewPluginRegistry(),
		connectors:              make(map[string]*ConnectorInstance),
		connectorRegistry:       NewConnectorRegistry(),
		nativePackages:          make(map[string]*InstalledPluginPackage),
		loadedNativePlugins:     make(map[string]string),
		nativeConnectorPackages: make(map[string]*InstalledConnectorPackage),
		loadedNativeConnectors:  make(map[string]string),
		db:                      db,
		eventManager:            eventManager,
		rconManager:             rconManager,
		clickhouseClient:        clickhouseClient,
		ctx:                     ctx,
		cancel:                  cancel,
	}

	// Subscribe to events for plugin distribution
	pm.eventSubscriber = pm.eventManager.Subscribe(event_manager.EventFilter{}, nil, 1000)

	return pm
}

// SetBanSyncFunc sets the callback used to regenerate Bans.cfg after a plugin-issued ban.
func (pm *PluginManager) SetBanSyncFunc(fn func(ctx context.Context, serverID uuid.UUID) error) {
	pm.banSyncFunc = fn
}

// Start starts the plugin manager
func (pm *PluginManager) Start() error {
	log.Info().Msg("Starting plugin manager")

	if nativePluginsEnabled() {
		// Force runtime dirs to resolve once at startup so the abs-path
		// warning fires before any uploads can race the cache, and so
		// containment checks remain stable across CWD changes.
		_ = pluginRuntimeDir()
		_ = connectorRuntimeDir()

		logSubprocessHardeningPosture()
	}

	if err := pm.loadInstalledPluginPackages(); err != nil {
		return fmt.Errorf("failed to load installed plugin packages: %w", err)
	}

	if err := pm.loadInstalledConnectorPackages(); err != nil {
		return fmt.Errorf("failed to load installed connector packages: %w", err)
	}

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
			log.Trace().
				Str("serverID", serverID.String()).
				Str("instanceID", instanceID.String()).
				Str("pluginID", instance.PluginID).
				Msg("Stopping plugin instance")
			if err := pm.stopPluginInstance(instance); err != nil {
				log.Error().
					Str("serverID", serverID.String()).
					Str("instanceID", instanceID.String()).
					Str("pluginID", instance.PluginID).
					Err(err).
					Msg("Failed to stop plugin instance")
			} else {
				log.Info().
					Str("serverID", serverID.String()).
					Str("instanceID", instanceID.String()).
					Str("pluginID", instance.PluginID).
					Msg("Stopped plugin instance")
			}

			// Check if plugin was forcefully killed due to timeout
			if instance.LastError == "Plugin shutdown timed out after 30 seconds" {
				log.Warn().
					Str("serverID", serverID.String()).
					Str("instanceID", instanceID.String()).
					Str("pluginID", instance.PluginID).
					Msg("Plugin instance was forcefully killed due to shutdown timeout")
			}
		}
	}
	pm.mu.Unlock()

	// Stop all connectors
	pm.connectorMu.Lock()
	for connectorID, instance := range pm.connectors {
		log.Trace().
			Str("connectorID", connectorID).
			Msg("Stopping connector instance")
		if err := pm.stopConnectorInstance(instance); err != nil {
			log.Error().
				Str("connectorID", connectorID).
				Err(err).
				Msg("Failed to stop connector instance")
		} else {
			log.Info().
				Str("connectorID", connectorID).
				Msg("Stopped connector instance")
		}

		// Check if connector was forcefully killed due to timeout
		if instance.LastError == "Connector shutdown timed out after 30 seconds" {
			log.Warn().
				Str("connectorID", connectorID).
				Msg("Connector instance was forcefully killed due to shutdown timeout")
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

// GetClickHouseClient returns the ClickHouse client instance
func (pm *PluginManager) GetClickHouseClient() *clickhouse.Client {
	return pm.clickhouseClient
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
func (pm *PluginManager) CreatePluginInstance(serverID uuid.UUID, pluginID string, notes string, config map[string]interface{}) (*PluginInstance, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Get plugin definition
	definition, err := pm.registry.GetPlugin(pluginID)
	if err != nil {
		return nil, fmt.Errorf("plugin not found: %w", err)
	}
	enrichedDefinition := pm.enrichPluginDefinition(*definition)
	if enrichedDefinition.Source == PluginSourceNative &&
		enrichedDefinition.InstallState != PluginInstallStateReady {
		return nil, fmt.Errorf("plugin %s is not ready to be enabled (state=%s)", pluginID, enrichedDefinition.InstallState)
	}

	// Validate config for creation (ensures sensitive required fields are provided)
	if err := enrichedDefinition.ConfigSchema.ValidateForCreation(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Fill defaults
	config = enrichedDefinition.ConfigSchema.FillDefaults(config)

	// Check if multiple instances are allowed
	if !enrichedDefinition.AllowMultipleInstances {
		if serverPlugins, exists := pm.plugins[serverID]; exists {
			for _, instance := range serverPlugins {
				if instance.PluginID == pluginID {
					return nil, fmt.Errorf("plugin %s does not allow multiple instances on server %s", pluginID, serverID.String())
				}
			}
		}
	}

	// Validate required connectors
	for _, connectorID := range enrichedDefinition.RequiredConnectors {
		storageKey, ok := pm.ResolveConnectorInstanceKey(connectorID)
		if !ok {
			return nil, fmt.Errorf("required connector %s is not available", connectorID)
		}
		pm.connectorMu.RLock()
		co := pm.connectors[storageKey]
		pm.connectorMu.RUnlock()
		if co == nil || co.Status != ConnectorStatusRunning {
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
		ID:                instanceID,
		ServerID:          serverID,
		PluginID:          pluginID,
		PluginName:        enrichedDefinition.Name,
		Source:            enrichedDefinition.Source,
		Official:          enrichedDefinition.Official,
		Distribution:      enrichedDefinition.Distribution,
		InstallState:      enrichedDefinition.InstallState,
		MinHostAPIVersion: enrichedDefinition.MinHostAPIVersion,
		Notes:             notes,
		Config:            config,
		Status:            PluginStatusStopped,
		Enabled:           true,
		LogLevel:          "info", // Default log level
		Plugin:            plugin,
		Context:           ctx,
		Cancel:            cancel,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
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
		if dbErr := pm.deletePluginInstanceFromDatabase(instanceID); dbErr != nil {
			log.Error().Err(dbErr).Str("instanceID", instanceID.String()).Msg("Failed to clean up plugin instance from database after init failure")
		}
		return nil, fmt.Errorf("failed to initialize plugin instance: %w", err)
	}

	log.Info().
		Str("serverID", serverID.String()).
		Str("instanceID", instanceID.String()).
		Str("pluginID", pluginID).
		Str("notes", notes).
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
		instances = append(instances, pm.maskAndEnrichPluginInstance(instance))
	}

	// Sort by created_at for stable ordering
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].CreatedAt.Before(instances[j].CreatedAt)
	})

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

	return pm.maskAndEnrichPluginInstance(instance), nil
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
		return fmt.Errorf("failed to delete plugin instance from database: %w", err)
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

	// Get plugin definition to validate config
	definition, err := pm.registry.GetPlugin(instance.PluginID)
	if err != nil {
		return fmt.Errorf("plugin definition not found: %w", err)
	}

	// Merge new config with existing, handling sensitive fields properly
	mergedConfig := definition.ConfigSchema.MergeConfigUpdates(instance.Config, config)

	// Validate the merged config
	if err := definition.ConfigSchema.Validate(mergedConfig); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Only call UpdateConfig on the plugin if it's initialized (enabled and running)
	// Disabled/stopped plugins haven't been initialized so their apis/dependencies are nil
	if instance.Status != PluginStatusDisabled && instance.Status != PluginStatusStopped {
		if err := instance.Plugin.UpdateConfig(mergedConfig); err != nil {
			return fmt.Errorf("failed to update plugin config: %w", err)
		}
	}

	// Update instance record
	instance.Config = mergedConfig
	instance.UpdatedAt = time.Now()

	// Save to database
	if err := pm.updatePluginInstanceInDatabase(instance); err != nil {
		return fmt.Errorf("failed to update plugin instance in database: %w", err)
	}

	return nil
}

// UpdatePluginLogLevel updates a plugin's log level
func (pm *PluginManager) UpdatePluginLogLevel(serverID, instanceID uuid.UUID, logLevel string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Validate log level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[logLevel] {
		return fmt.Errorf("invalid log level: %s (must be one of: debug, info, warn, error)", logLevel)
	}

	instance, err := pm.getPluginInstanceUnsafe(serverID, instanceID)
	if err != nil {
		return err
	}

	// Update instance record
	instance.LogLevel = logLevel
	instance.UpdatedAt = time.Now()

	// Save to database
	if err := pm.updatePluginInstanceInDatabase(instance); err != nil {
		return fmt.Errorf("failed to update plugin instance in database: %w", err)
	}

	// Restart plugin to apply new log level
	if instance.Enabled && instance.Status == PluginStatusRunning {
		if err := pm.stopPluginInstance(instance); err != nil {
			log.Error().
				Str("serverID", serverID.String()).
				Str("instanceID", instanceID.String()).
				Err(err).
				Msg("Failed to stop plugin instance after log level update")
		}

		if err := pm.initializePluginInstance(instance); err != nil {
			log.Error().
				Str("serverID", serverID.String()).
				Str("instanceID", instanceID.String()).
				Err(err).
				Msg("Failed to restart plugin instance after log level update")
			instance.setError(PluginStatusError, err.Error())
			return fmt.Errorf("failed to restart plugin instance: %w", err)
		}
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

	// Check if plugin was forcefully killed due to timeout
	if instance.LastError == "Plugin shutdown timed out after 30 seconds" {
		log.Warn().
			Str("serverID", serverID.String()).
			Str("instanceID", instanceID.String()).
			Str("pluginID", instance.PluginID).
			Msg("Plugin instance was forcefully killed due to shutdown timeout during disable")
	}

	instance.Enabled = false
	instance.setStatus(PluginStatusDisabled)
	instance.UpdatedAt = time.Now()

	// Save to database
	if err := pm.updatePluginInstanceInDatabase(instance); err != nil {
		return fmt.Errorf("failed to update plugin instance in database: %w", err)
	}

	return nil
}

// getDiscordAPI returns the Discord connector API for use by plugins when available.
func (pm *PluginManager) getDiscordAPI() DiscordAPI {
	storageKey, ok := pm.ResolveConnectorInstanceKey("com.squad-aegis.connectors.discord")
	if !ok {
		return nil
	}

	pm.connectorMu.RLock()
	defer pm.connectorMu.RUnlock()

	instance := pm.connectors[storageKey]
	if instance == nil {
		return nil
	}

	if instance.Status != ConnectorStatusRunning {
		return nil
	}

	api, ok := instance.Connector.GetAPI().(DiscordAPI)
	if !ok {
		log.Error().Msg("Discord connector API does not implement the plugin DiscordAPI interface")
		return nil
	}

	return api
}

// ListAvailablePlugins returns all available plugin definitions
func (pm *PluginManager) ListAvailablePlugins() []PluginDefinition {
	definitions := pm.registry.ListPlugins()
	available := make([]PluginDefinition, 0, len(definitions))

	for _, definition := range definitions {
		enriched := pm.enrichPluginDefinition(definition)
		if enriched.Source == PluginSourceNative &&
			enriched.InstallState != PluginInstallStateReady {
			continue
		}
		available = append(available, enriched)
	}

	sort.Slice(available, func(i, j int) bool {
		return available[i].Name < available[j].Name
	})

	return available
}

// ListAvailableConnectorDefinitions returns all available connector definitions
func (pm *PluginManager) ListAvailableConnectorDefinitions() []ConnectorDefinition {
	defs := pm.connectorRegistry.ListConnectors()
	out := make([]ConnectorDefinition, 0, len(defs))
	for _, d := range defs {
		out = append(out, pm.enrichConnectorDefinition(d))
	}
	return out
}

// GetConnectors returns all connector instances
func (pm *PluginManager) GetConnectors() []*ConnectorInstance {
	pm.connectorMu.RLock()
	defer pm.connectorMu.RUnlock()

	connectors := make([]*ConnectorInstance, 0, len(pm.connectors))
	for _, instance := range pm.connectors {
		// Build a shallow copy field-by-field to avoid copying the sync.Mutex,
		// and take the lock so Status/LastError are read atomically with writes
		// from the unexpected-exit reporter.
		instance.mu.Lock()
		maskedInstance := &ConnectorInstance{
			ID:        instance.ID,
			Config:    instance.Config,
			Status:    instance.Status,
			Enabled:   instance.Enabled,
			Connector: instance.Connector,
			Context:   instance.Context,
			Cancel:    instance.Cancel,
			LastError: instance.LastError,
			CreatedAt: instance.CreatedAt,
			UpdatedAt: instance.UpdatedAt,
		}
		instance.mu.Unlock()
		if definition, err := pm.connectorRegistry.GetConnector(instance.ID); err == nil {
			maskedInstance.Config = definition.ConfigSchema.MaskSensitiveFields(instance.Config)
		}
		connectors = append(connectors, maskedInstance)
	}

	return connectors
}

// pluginDependsOnConnectorRef returns true if the plugin lists connectorRef as a required or optional connector instance.
func (pm *PluginManager) pluginDependsOnConnectorRef(definition *PluginDefinition, connectorRef string) bool {
	if definition == nil {
		return false
	}
	updatedKey, okUpdated := pm.ResolveConnectorInstanceKey(connectorRef)
	if !okUpdated {
		updatedKey = strings.TrimSpace(connectorRef)
	}
	if updatedKey == "" {
		return false
	}
	refs := make([]string, 0, len(definition.RequiredConnectors)+len(definition.OptionalConnectors))
	refs = append(refs, definition.RequiredConnectors...)
	refs = append(refs, definition.OptionalConnectors...)
	for _, c := range refs {
		reqKey, okReq := pm.ResolveConnectorInstanceKey(c)
		if !okReq {
			reqKey = strings.TrimSpace(c)
		}
		if reqKey != "" && reqKey == updatedKey {
			return true
		}
	}
	return false
}

// restartDependentPlugins restarts all plugins that depend on a specific connector
func (pm *PluginManager) restartDependentPlugins(connectorID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var restartErrors []error

	// Iterate through all servers and their plugins
	for serverID, serverPlugins := range pm.plugins {
		for instanceID, instance := range serverPlugins {
			// Get plugin definition to check required connectors
			definition, err := pm.registry.GetPlugin(instance.PluginID)
			if err != nil {
				log.Error().
					Str("serverID", serverID.String()).
					Str("instanceID", instanceID.String()).
					Str("pluginID", instance.PluginID).
					Err(err).
					Msg("Failed to get plugin definition while checking connector dependencies")
				continue
			}

			dependsOnConnector := pm.pluginDependsOnConnectorRef(definition, connectorID)

			if !dependsOnConnector {
				continue // Skip plugins that don't depend on this connector
			}

			// Only restart if the plugin is currently enabled and running
			if !instance.Enabled || instance.Status != PluginStatusRunning {
				continue
			}

			log.Info().
				Str("serverID", serverID.String()).
				Str("instanceID", instanceID.String()).
				Str("pluginID", instance.PluginID).
				Str("connectorID", connectorID).
				Msg("Restarting plugin due to connector update")

			// Stop the plugin
			if err := pm.stopPluginInstance(instance); err != nil {
				log.Error().
					Str("serverID", serverID.String()).
					Str("instanceID", instanceID.String()).
					Str("pluginID", instance.PluginID).
					Str("connectorID", connectorID).
					Err(err).
					Msg("Failed to stop plugin instance during connector update restart")
				restartErrors = append(restartErrors, fmt.Errorf("failed to stop plugin %s: %w", instanceID.String(), err))
				continue
			}

			// Restart the plugin (reinitialize and start)
			if err := pm.initializePluginInstance(instance); err != nil {
				log.Error().
					Str("serverID", serverID.String()).
					Str("instanceID", instanceID.String()).
					Str("pluginID", instance.PluginID).
					Str("connectorID", connectorID).
					Err(err).
					Msg("Failed to restart plugin instance after connector update")
				restartErrors = append(restartErrors, fmt.Errorf("failed to restart plugin %s: %w", instanceID.String(), err))
				continue
			}

			log.Info().
				Str("serverID", serverID.String()).
				Str("instanceID", instanceID.String()).
				Str("pluginID", instance.PluginID).
				Str("connectorID", connectorID).
				Msg("Successfully restarted plugin after connector update")
		}
	}

	// Return combined errors if any occurred
	if len(restartErrors) > 0 {
		return fmt.Errorf("encountered %d errors while restarting dependent plugins: %v", len(restartErrors), restartErrors)
	}

	return nil
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

func (pm *PluginManager) ensurePluginInstanceRuntime(instance *PluginInstance) error {
	if instance.Plugin != nil {
		pm.ensurePluginInstanceContext(instance)
		return nil
	}

	definition, err := pm.registry.GetPlugin(instance.PluginID)
	if err != nil {
		return fmt.Errorf("plugin definition unavailable: %w", err)
	}

	enrichedDefinition := pm.enrichPluginDefinition(*definition)
	pm.applyPluginDefinitionMetadata(instance, enrichedDefinition)
	if enrichedDefinition.Source == PluginSourceNative &&
		enrichedDefinition.InstallState != PluginInstallStateReady {
		return fmt.Errorf("plugin %s is not ready to be enabled (state=%s)", instance.PluginID, enrichedDefinition.InstallState)
	}

	plugin, err := pm.registry.CreatePluginInstance(instance.PluginID)
	if err != nil {
		return fmt.Errorf("failed to create plugin instance: %w", err)
	}

	pm.ensurePluginInstanceContext(instance)
	instance.Plugin = plugin
	return nil
}

// safePluginCall wraps an arbitrary plugin lifecycle call with a recover so a
// panicking native plugin cannot crash the entire PluginManager process. The
// returned error is non-nil if the call panicked or returned an error.
func safePluginCall(pluginID string, op string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := make([]byte, 4096)
			n := runtime.Stack(stack, false)
			log.Error().
				Str("plugin_id", pluginID).
				Str("operation", op).
				Interface("panic", r).
				Bytes("stack", stack[:n]).
				Msg("Plugin lifecycle call panicked; isolating failure")
			err = fmt.Errorf("plugin %s panicked in %s: %v", pluginID, op, r)
		}
	}()
	return fn()
}

// unexpectedExitReporter is the narrow interface subprocess shims expose so
// the plugin manager can be notified when their subprocess crashes.
type unexpectedExitReporter interface {
	OnUnexpectedExit(func(error))
}

func (pm *PluginManager) initializePluginInstance(instance *PluginInstance) error {
	if err := pm.ensurePluginInstanceRuntime(instance); err != nil {
		instance.setError(PluginStatusError, err.Error())
		return err
	}

	instance.setStatus(PluginStatusStarting)

	// Create plugin APIs
	apis := pm.createPluginAPIs(instance.Context, instance.ServerID, instance.ID, instance.PluginName, instance.PluginID, instance.LogLevel)

	// Initialize plugin (with panic recovery so a misbehaving native .so
	// cannot take down the manager).
	if err := safePluginCall(instance.PluginID, "Initialize", func() error {
		return instance.Plugin.Initialize(instance.Config, apis)
	}); err != nil {
		instance.setError(PluginStatusError, err.Error())
		return fmt.Errorf("failed to initialize plugin: %w", err)
	}

	// Wire unexpected-exit reporting for subprocess-isolated plugins. A
	// crashing subprocess flips the instance to error state so operators
	// can see the failure on the server's plugin page.
	pm.attachUnexpectedExitReporter(instance)

	// Start plugin if it's long-running
	definition := instance.Plugin.GetDefinition()
	if definition.LongRunning {
		if err := safePluginCall(instance.PluginID, "Start", func() error {
			return instance.Plugin.Start(instance.Context)
		}); err != nil {
			// Start failed after a successful Initialize. Tear down the
			// plugin so any subprocess/goroutine/RPC listener spawned by
			// Initialize is released; otherwise the instance is wedged and
			// the next Enable hits "plugin subprocess already initialized".
			if stopErr := safePluginCall(instance.PluginID, "Stop", instance.Plugin.Stop); stopErr != nil {
				log.Warn().
					Err(stopErr).
					Str("plugin_id", instance.PluginID).
					Str("instance_id", instance.ID.String()).
					Msg("Failed to stop plugin after Start error; subprocess may leak")
			}
			instance.setError(PluginStatusError, err.Error())
			return fmt.Errorf("failed to start plugin: %w", err)
		}
	}

	instance.clearError(PluginStatusRunning)
	return nil
}

// attachUnexpectedExitReporter wires the instance-level error state into the
// shim's health watcher so a crashing subprocess automatically marks the
// instance as errored. No-op if the plugin isn't subprocess-isolated.
//
// The callback must not acquire pm.mu: if it did, it would deadlock when an
// admin caller (Delete/Disable/UpdateLogLevel) holds pm.mu while waiting on
// Plugin.Stop(), which itself waits on the watcher goroutine to exit. The
// captured `instance` pointer stays valid for the lifetime of the closure,
// and instance.setError/setStatus are synchronized via instance.mu.
func (pm *PluginManager) attachUnexpectedExitReporter(instance *PluginInstance) {
	reporter, ok := instance.Plugin.(unexpectedExitReporter)
	if !ok {
		return
	}
	pluginID := instance.PluginID
	instanceID := instance.ID
	serverID := instance.ServerID
	reporter.OnUnexpectedExit(func(err error) {
		if err != nil {
			instance.setError(PluginStatusError, err.Error())
		} else {
			instance.setStatus(PluginStatusError)
		}
		log.Warn().
			Err(err).
			Str("plugin_id", pluginID).
			Str("instance_id", instanceID.String()).
			Str("server_id", serverID.String()).
			Msg("Plugin subprocess exited unexpectedly; instance marked as errored")
	})
}

func (pm *PluginManager) stopPluginInstance(instance *PluginInstance) error {
	if instance.Status != PluginStatusRunning {
		return nil // Not running, nothing to do
	}
	if instance.Plugin == nil {
		instance.setStatus(PluginStatusStopped)
		return nil
	}

	instance.setStatus(PluginStatusStopping)

	// Cancel context
	if instance.Cancel != nil {
		instance.Cancel()
	}

	// Stop plugin with 30-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stopChan := make(chan error, 1)

	go func() {
		stopChan <- instance.Plugin.Stop()
	}()

	select {
	case err := <-stopChan:
		if err != nil {
			instance.setError(PluginStatusError, err.Error())
			return fmt.Errorf("failed to stop plugin: %w", err)
		}
		instance.setStatus(PluginStatusStopped)
		return nil
	case <-ctx.Done():
		log.Warn().
			Str("pluginID", instance.PluginID).
			Str("instanceID", instance.ID.String()).
			Msg("Plugin shutdown timed out after 30 seconds, forcefully killing it")
		instance.setError(PluginStatusStopped, "Plugin shutdown timed out after 30 seconds")
		return nil
	}
}

func (pm *PluginManager) createPluginAPIs(ctx context.Context, serverID, instanceID uuid.UUID, pluginName, pluginID, logLevel string) *PluginAPIs {
	apis := &PluginAPIs{}
	if pm.shouldExposePluginAPI(pluginID, NativePluginCapabilityAPIServer) {
		apis.ServerAPI = NewServerAPI(serverID, pm.db, pm.rconManager)
	}
	if pm.shouldExposePluginAPI(pluginID, NativePluginCapabilityAPIDatabase) {
		apis.DatabaseAPI = NewDatabaseAPI(instanceID, pm.db)
	}
	if pm.shouldExposePluginAPI(pluginID, NativePluginCapabilityAPIRule) {
		apis.RuleAPI = NewRuleAPI(serverID, pm.db)
	}
	if pm.shouldExposePluginAPI(pluginID, NativePluginCapabilityAPIRCON) {
		apis.RconAPI = NewRconAPI(serverID, pm.db, pm.rconManager, pm.clickhouseClient, pm.banSyncFunc)
	}
	if pm.shouldExposePluginAPI(pluginID, NativePluginCapabilityAPIAdmin) {
		apis.AdminAPI = NewAdminAPI(serverID, pm.db, pm.rconManager, instanceID)
	}
	if pm.shouldExposePluginAPI(pluginID, NativePluginCapabilityAPIEvent) {
		apis.EventAPI = NewEventAPI(ctx, serverID, instanceID, pluginName, pm.eventManager)
	}
	if pm.shouldExposePluginAPI(pluginID, NativePluginCapabilityAPIDiscord) {
		apis.DiscordAPI = pm.getDiscordAPI()
	}
	if pm.shouldExposePluginAPI(pluginID, NativePluginCapabilityAPILog) {
		apis.LogAPI = NewLogAPI(serverID, instanceID, pluginName, pluginID, logLevel, pm.clickhouseClient, pm.db, pm.eventManager)
	}
	if pm.shouldExposeConnectorAPI(pluginID) {
		apis.ConnectorAPI = newConnectorAPI(pm)
	}
	return apis
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
			for _, e := range definition.Events {
				if e == event.Type || e == event_manager.EventTypeAll {
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

			instance.setError(PluginStatusError, fmt.Sprintf("panic: %v", r))
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

		instance.mu.Lock()
		instance.LastError = err.Error()
		instance.mu.Unlock()
	}
}

func (pm *PluginManager) convertEventSource(eventType event_manager.EventType) EventSource {
	eventTypeStr := string(eventType)
	switch {
	case strings.HasPrefix(eventTypeStr, "RCON"):
		return EventSourceRCON
	case strings.HasPrefix(eventTypeStr, "LOG"):
		return EventSourceLog
	default:
		return EventSourceSystem
	}
}

// PluginLog represents a log entry from ClickHouse
type PluginLog struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	Level        string                 `json:"level"`
	Message      string                 `json:"message"`
	ErrorMessage *string                `json:"error_message,omitempty"`
	Fields       map[string]interface{} `json:"fields,omitempty"`
	IngestedAt   time.Time              `json:"ingested_at"`
}

// GetPluginLogs retrieves logs for a specific plugin instance from ClickHouse
func (pm *PluginManager) GetPluginLogs(serverID, instanceID uuid.UUID, limit int, before, after, order, level, search string) ([]PluginLog, error) {
	if pm.clickhouseClient == nil {
		return nil, fmt.Errorf("ClickHouse client not available")
	}

	// Default limit if not specified
	if limit <= 0 {
		limit = 100
	}

	// Prevent excessively large queries
	if limit > 1000 {
		limit = 1000
	}

	// Build the query
	query := "SELECT log_id, timestamp, level, message, error_message, fields, ingested_at FROM squad_aegis.plugin_logs WHERE server_id = ? AND plugin_instance_id = ?"
	args := []interface{}{serverID, instanceID}

	if level != "" && level != "all" {
		query += " AND level = ?"
		args = append(args, level)
	}

	if search != "" {
		query += " AND (message LIKE ? OR error_message LIKE ?)"
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Handle cursor-based pagination
	if before != "" {
		var beforeTimestamp time.Time
		tsQuery := "SELECT timestamp FROM squad_aegis.plugin_logs WHERE log_id = ? AND server_id = ? AND plugin_instance_id = ?"
		err := pm.clickhouseClient.QueryRow(ctx, tsQuery, before, serverID, instanceID).Scan(&beforeTimestamp)
		if err != nil {
			if err == sql.ErrNoRows {
				return []PluginLog{}, nil // If cursor not found, return no older logs
			}
			return nil, fmt.Errorf("failed to get timestamp for 'before' cursor: %w", err)
		}
		query += " AND (timestamp, log_id) < (?, ?)"
		args = append(args, beforeTimestamp, before)
	}

	if after != "" {
		var afterTimestamp time.Time
		tsQuery := "SELECT timestamp FROM squad_aegis.plugin_logs WHERE log_id = ? AND server_id = ? AND plugin_instance_id = ?"
		err := pm.clickhouseClient.QueryRow(ctx, tsQuery, after, serverID, instanceID).Scan(&afterTimestamp)
		if err != nil {
			if err == sql.ErrNoRows {
				return []PluginLog{}, nil // If cursor not found, return no newer logs
			}
			return nil, fmt.Errorf("failed to get timestamp for 'after' cursor: %w", err)
		}
		query += " AND (timestamp, log_id) > (?, ?)"
		args = append(args, afterTimestamp, after)
	}

	// Handle order
	if order == "desc" {
		query += " ORDER BY timestamp DESC, log_id DESC"
	} else {
		query += " ORDER BY timestamp ASC, log_id ASC"
	}

	query += " LIMIT ?"
	args = append(args, limit)

	rows, err := pm.clickhouseClient.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query plugin logs: %w", err)
	}
	defer rows.Close()

	var logs []PluginLog
	for rows.Next() {
		var logItem PluginLog
		var fieldsJSON string
		var errorMessage sql.NullString

		err := rows.Scan(
			&logItem.ID,
			&logItem.Timestamp,
			&logItem.Level,
			&logItem.Message,
			&errorMessage,
			&fieldsJSON,
			&logItem.IngestedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log row: %w", err)
		}

		if errorMessage.Valid {
			logItem.ErrorMessage = &errorMessage.String
		}

		// Parse fields JSON
		if fieldsJSON != "" {
			var fields map[string]interface{}
			if err := json.Unmarshal([]byte(fieldsJSON), &fields); err == nil {
				logItem.Fields = fields
			} else {
				logItem.Fields = map[string]interface{}{"raw": fieldsJSON, "error": "failed to parse json"}
			}
		}

		logs = append(logs, logItem)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over log rows: %w", err)
	}

	return logs, nil
}

// AggregatedPluginLog extends PluginLog with plugin information
type AggregatedPluginLog struct {
	PluginLog
	PluginInstanceID uuid.UUID `json:"plugin_instance_id"`
	PluginName       string    `json:"plugin_name"`
	PluginID         string    `json:"plugin_id"`
}

// GetServerPluginLogs retrieves logs for all plugin instances for a specific server from ClickHouse
func (pm *PluginManager) GetServerPluginLogs(serverID uuid.UUID, limit int, before, after, order, level, search string) ([]AggregatedPluginLog, error) {
	if pm.clickhouseClient == nil {
		return nil, fmt.Errorf("ClickHouse client not available")
	}

	// Default limit if not specified
	if limit <= 0 {
		limit = 100
	}

	// Prevent excessively large queries
	if limit > 1000 {
		limit = 1000
	}

	// Build the query - aggregate logs from all plugins for this server
	query := `SELECT
		log_id,
		timestamp,
		level,
		message,
		error_message,
		fields,
		ingested_at,
		plugin_instance_id
	FROM squad_aegis.plugin_logs
	WHERE server_id = ?`
	args := []interface{}{serverID}

	if level != "" && level != "all" {
		query += " AND level = ?"
		args = append(args, level)
	}

	if search != "" {
		query += " AND (message LIKE ? OR error_message LIKE ?)"
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Handle cursor-based pagination
	if before != "" {
		var beforeTimestamp time.Time
		tsQuery := "SELECT timestamp FROM squad_aegis.plugin_logs WHERE log_id = ? AND server_id = ?"
		err := pm.clickhouseClient.QueryRow(ctx, tsQuery, before, serverID).Scan(&beforeTimestamp)
		if err != nil {
			if err == sql.ErrNoRows {
				return []AggregatedPluginLog{}, nil // If cursor not found, return no older logs
			}
			return nil, fmt.Errorf("failed to get timestamp for 'before' cursor: %w", err)
		}
		query += " AND (timestamp, log_id) < (?, ?)"
		args = append(args, beforeTimestamp, before)
	}

	if after != "" {
		var afterTimestamp time.Time
		tsQuery := "SELECT timestamp FROM squad_aegis.plugin_logs WHERE log_id = ? AND server_id = ?"
		err := pm.clickhouseClient.QueryRow(ctx, tsQuery, after, serverID).Scan(&afterTimestamp)
		if err != nil {
			if err == sql.ErrNoRows {
				return []AggregatedPluginLog{}, nil // If cursor not found, return no newer logs
			}
			return nil, fmt.Errorf("failed to get timestamp for 'after' cursor: %w", err)
		}
		query += " AND (timestamp, log_id) > (?, ?)"
		args = append(args, afterTimestamp, after)
	}

	// Handle order
	if order == "desc" {
		query += " ORDER BY timestamp DESC, log_id DESC"
	} else {
		query += " ORDER BY timestamp ASC, log_id ASC"
	}

	query += " LIMIT ?"
	args = append(args, limit)

	rows, err := pm.clickhouseClient.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query server plugin logs: %w", err)
	}
	defer rows.Close()

	var logs []AggregatedPluginLog
	for rows.Next() {
		var logItem AggregatedPluginLog
		var fieldsJSON string
		var errorMessage sql.NullString

		err := rows.Scan(
			&logItem.ID,
			&logItem.Timestamp,
			&logItem.Level,
			&logItem.Message,
			&errorMessage,
			&fieldsJSON,
			&logItem.IngestedAt,
			&logItem.PluginInstanceID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log row: %w", err)
		}

		if errorMessage.Valid {
			logItem.ErrorMessage = &errorMessage.String
		}

		// Parse fields JSON
		if fieldsJSON != "" {
			var fields map[string]interface{}
			if err := json.Unmarshal([]byte(fieldsJSON), &fields); err == nil {
				logItem.Fields = fields
			} else {
				logItem.Fields = map[string]interface{}{"raw": fieldsJSON, "error": "failed to parse json"}
			}
		}

		// Get plugin information
		pm.mu.RLock()
		if serverPlugins, exists := pm.plugins[serverID]; exists {
			if instance, exists := serverPlugins[logItem.PluginInstanceID]; exists {
				if instance.Plugin != nil {
					logItem.PluginName = instance.Plugin.GetDefinition().Name
					logItem.PluginID = instance.Plugin.GetDefinition().ID
				} else {
					logItem.PluginName = instance.PluginID
					logItem.PluginID = instance.PluginID
				}
			} else {
				// Fallback: try to get plugin info from database or use instance ID
				logItem.PluginName = "Unknown Plugin"
				logItem.PluginID = logItem.PluginInstanceID.String()
			}
		} else {
			logItem.PluginName = "Unknown Plugin"
			logItem.PluginID = logItem.PluginInstanceID.String()
		}
		pm.mu.RUnlock()

		logs = append(logs, logItem)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over log rows: %w", err)
	}

	return logs, nil
}

// GetPluginCommands returns available commands for a plugin instance
func (pm *PluginManager) GetPluginCommands(serverID, instanceID uuid.UUID) ([]PluginCommand, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	instance, err := pm.getPluginInstanceUnsafe(serverID, instanceID)
	if err != nil {
		return nil, err
	}

	if instance.Plugin == nil {
		return nil, fmt.Errorf("plugin instance not initialized")
	}

	// Get commands from plugin
	commands := instance.Plugin.GetCommands()
	return commands, nil
}

// ExecutePluginCommand executes a command on a plugin instance
func (pm *PluginManager) ExecutePluginCommand(serverID, instanceID uuid.UUID, commandID string, params map[string]interface{}) (*CommandResult, error) {
	pm.mu.RLock()
	instance, err := pm.getPluginInstanceUnsafe(serverID, instanceID)
	pm.mu.RUnlock()

	if err != nil {
		return nil, err
	}

	if instance.Plugin == nil {
		return nil, fmt.Errorf("plugin instance not initialized")
	}

	if instance.Status != PluginStatusRunning || !instance.Enabled {
		return nil, fmt.Errorf("plugin instance is not running")
	}

	// Validate command exists
	commands := instance.Plugin.GetCommands()
	var command *PluginCommand
	for i := range commands {
		if commands[i].ID == commandID {
			command = &commands[i]
			break
		}
	}

	if command == nil {
		return nil, fmt.Errorf("command %s not found", commandID)
	}

	// Validate parameters if schema is defined
	if len(command.Parameters.Fields) > 0 {
		if err := command.Parameters.Validate(params); err != nil {
			return nil, fmt.Errorf("parameter validation failed: %w", err)
		}
	}

	// Execute command with panic recovery
	var result *CommandResult
	var execErr error

	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Str("serverID", serverID.String()).
					Str("instanceID", instanceID.String()).
					Str("pluginID", instance.PluginID).
					Str("commandID", commandID).
					Interface("panic", r).
					Msg("Plugin panicked while executing command")

				execErr = fmt.Errorf("plugin panicked: %v", r)
			}
		}()

		result, execErr = instance.Plugin.ExecuteCommand(commandID, params)
	}()

	if execErr != nil {
		return nil, execErr
	}

	return result, nil
}

// GetCommandExecutionStatus gets the status of an async command execution
func (pm *PluginManager) GetCommandExecutionStatus(serverID, instanceID uuid.UUID, executionID string) (*CommandExecutionStatus, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	instance, err := pm.getPluginInstanceUnsafe(serverID, instanceID)
	if err != nil {
		return nil, err
	}

	if instance.Plugin == nil {
		return nil, fmt.Errorf("plugin instance not initialized")
	}

	// Get execution status from plugin
	status, err := instance.Plugin.GetCommandExecutionStatus(executionID)
	if err != nil {
		return nil, err
	}

	return status, nil
}
