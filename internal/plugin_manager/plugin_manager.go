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
		startRevokedKeyIDsRefresher(pm.ctx)
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

// Stop stops the plugin manager.
//
// Intentionally holds pm.mu / pm.connectorMu for the entire shutdown sweep.
// At this point pm.cancel() has already fired, so any pending lifecycle
// operations should bail out early; serializing the shutdown under the
// registry locks prevents new admin operations from racing with the
// per-instance Stop RPCs and prevents a delete handler from removing an
// entry the sweep is mid-iterating.
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
	// Get plugin definition (registry has its own lock; no pm.mu needed here)
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

	// Validate required connectors are running. ResolveConnectorInstanceKey
	// and the snapshot read take the connectorMu briefly and release it before
	// any RPC happens.
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

	// Reserve the instance slot under pm.mu (multi-instance check + insert)
	// before doing any subprocess RPC so we can detect concurrent creates,
	// then release pm.mu while running Initialize/Start. Holding pm.mu across
	// an Initialize RPC (which can take seconds and on failure up to 30s)
	// would block every other plugin operation.
	plugin, err := pm.registry.CreatePluginInstance(pluginID)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin instance: %w", err)
	}

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

	pm.mu.Lock()
	if !enrichedDefinition.AllowMultipleInstances {
		if serverPlugins, exists := pm.plugins[serverID]; exists {
			for _, existing := range serverPlugins {
				if existing.PluginID == pluginID {
					pm.mu.Unlock()
					cancel()
					return nil, fmt.Errorf("plugin %s does not allow multiple instances on server %s", pluginID, serverID.String())
				}
			}
		}
	}
	if pm.plugins[serverID] == nil {
		pm.plugins[serverID] = make(map[uuid.UUID]*PluginInstance)
	}
	pm.plugins[serverID][instanceID] = instance
	pm.mu.Unlock()

	// Save to database (no pm.mu held; instance is reserved in the map but
	// Status is Stopped so no one can route events to it yet).
	if err := pm.savePluginInstanceToDatabase(instance); err != nil {
		pm.mu.Lock()
		if serverPlugins, ok := pm.plugins[serverID]; ok {
			delete(serverPlugins, instanceID)
			if len(serverPlugins) == 0 {
				delete(pm.plugins, serverID)
			}
		}
		pm.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("failed to save plugin instance to database: %w", err)
	}

	// Initialize and start plugin without holding pm.mu. The call may RPC
	// into a subprocess and take seconds; blocking pm.mu would stall every
	// other plugin/event operation.
	if err := pm.initializePluginInstance(instance); err != nil {
		pm.mu.Lock()
		if serverPlugins, ok := pm.plugins[serverID]; ok {
			delete(serverPlugins, instanceID)
			if len(serverPlugins) == 0 {
				delete(pm.plugins, serverID)
			}
		}
		pm.mu.Unlock()
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
	// Snapshot the instance under pm.mu, then release before the Stop RPC
	// (which may take up to 30s on a misbehaving plugin). Holding pm.mu
	// across that call would block every other plugin operation.
	pm.mu.Lock()
	serverPlugins, exists := pm.plugins[serverID]
	if !exists {
		pm.mu.Unlock()
		return fmt.Errorf("no plugins found for server %s", serverID.String())
	}
	instance, exists := serverPlugins[instanceID]
	if !exists {
		pm.mu.Unlock()
		return fmt.Errorf("plugin instance %s not found", instanceID.String())
	}
	pm.mu.Unlock()

	// Stop plugin (may RPC into subprocess; performed without pm.mu held).
	if err := pm.stopPluginInstance(instance); err != nil {
		log.Error().
			Str("serverID", serverID.String()).
			Str("instanceID", instanceID.String()).
			Err(err).
			Msg("Failed to stop plugin instance during deletion")
	}

	// Remove from database. Done before re-acquiring pm.mu so we don't block
	// readers on a slow database round trip either.
	if err := pm.deletePluginInstanceFromDatabase(instanceID); err != nil {
		log.Error().
			Str("instanceID", instanceID.String()).
			Err(err).
			Msg("Failed to delete plugin instance from database")
		return fmt.Errorf("failed to delete plugin instance from database: %w", err)
	}

	// Re-acquire the lock briefly to commit the removal. If a concurrent
	// caller has already removed or replaced this entry we still treat the
	// delete as successful (the instance is gone and the DB row is gone).
	pm.mu.Lock()
	if serverPlugins, ok := pm.plugins[serverID]; ok {
		if current, ok := serverPlugins[instanceID]; ok && current == instance {
			delete(serverPlugins, instanceID)
			if len(serverPlugins) == 0 {
				delete(pm.plugins, serverID)
			}
		}
	}
	pm.mu.Unlock()

	log.Info().
		Str("serverID", serverID.String()).
		Str("instanceID", instanceID.String()).
		Str("pluginID", instance.PluginID).
		Msg("Deleted plugin instance")

	return nil
}

// UpdatePluginConfig updates a plugin's configuration
func (pm *PluginManager) UpdatePluginConfig(serverID, instanceID uuid.UUID, config map[string]interface{}) error {
	// Snapshot the instance and validate the new config under pm.mu, then
	// release before calling Plugin.UpdateConfig (an RPC on native plugins).
	// Holding pm.mu across that call would block every other plugin
	// operation for seconds.
	pm.mu.Lock()
	instance, err := pm.getPluginInstanceUnsafe(serverID, instanceID)
	if err != nil {
		pm.mu.Unlock()
		return err
	}

	definition, err := pm.registry.GetPlugin(instance.PluginID)
	if err != nil {
		pm.mu.Unlock()
		return fmt.Errorf("plugin definition not found: %w", err)
	}

	mergedConfig := definition.ConfigSchema.MergeConfigUpdates(instance.Config, config)
	if err := definition.ConfigSchema.Validate(mergedConfig); err != nil {
		pm.mu.Unlock()
		return fmt.Errorf("config validation failed: %w", err)
	}

	plugin := instance.Plugin
	// Status is read under pm.mu so the live/initialized check is consistent
	// with the snapshot of `plugin` above.
	statusAtSnapshot := instance.Status
	pm.mu.Unlock()

	// Only call UpdateConfig on the plugin if it's initialized (enabled and
	// running). Disabled/stopped plugins haven't been initialized so their
	// apis/dependencies are nil. The call may take seconds on a subprocess
	// plugin; running it without pm.mu held lets other operations proceed.
	if plugin != nil && statusAtSnapshot != PluginStatusDisabled && statusAtSnapshot != PluginStatusStopped {
		if err := plugin.UpdateConfig(mergedConfig); err != nil {
			return fmt.Errorf("failed to update plugin config: %w", err)
		}
	}

	// Re-acquire the lock to commit the new config. If the instance was
	// deleted or replaced while we were doing RPC, discard our changes so a
	// stale config doesn't overwrite a fresh entry.
	pm.mu.Lock()
	defer pm.mu.Unlock()

	current, err := pm.getPluginInstanceUnsafe(serverID, instanceID)
	if err != nil {
		return fmt.Errorf("plugin instance %s was modified concurrently; config update discarded", instanceID.String())
	}
	if current != instance {
		return fmt.Errorf("plugin instance %s was replaced concurrently; config update discarded", instanceID.String())
	}

	instance.Config = mergedConfig
	instance.UpdatedAt = time.Now()

	if err := pm.updatePluginInstanceInDatabase(instance); err != nil {
		return fmt.Errorf("failed to update plugin instance in database: %w", err)
	}

	return nil
}

// UpdatePluginLogLevel updates a plugin's log level
func (pm *PluginManager) UpdatePluginLogLevel(serverID, instanceID uuid.UUID, logLevel string) error {
	// Validate log level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[logLevel] {
		return fmt.Errorf("invalid log level: %s (must be one of: debug, info, warn, error)", logLevel)
	}

	// Snapshot the instance under pm.mu, persist the new level, then release
	// the lock before stop+init RPCs which can take up to 30s on a misbehaving
	// plugin. Holding pm.mu across those RPCs would block every other plugin
	// operation.
	pm.mu.Lock()
	instance, err := pm.getPluginInstanceUnsafe(serverID, instanceID)
	if err != nil {
		pm.mu.Unlock()
		return err
	}

	instance.LogLevel = logLevel
	instance.UpdatedAt = time.Now()

	if err := pm.updatePluginInstanceInDatabase(instance); err != nil {
		pm.mu.Unlock()
		return fmt.Errorf("failed to update plugin instance in database: %w", err)
	}

	needsRestart := instance.Enabled && instance.Status == PluginStatusRunning
	pm.mu.Unlock()

	if !needsRestart {
		return nil
	}

	// Restart plugin to apply new log level (no pm.mu held during RPC).
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

	return nil
}

// EnablePluginInstance enables a plugin instance
func (pm *PluginManager) EnablePluginInstance(serverID, instanceID uuid.UUID) error {
	// Mark the instance as enabled under pm.mu, then release before
	// initializePluginInstance which RPCs into the subprocess. Holding pm.mu
	// across that call would block every other plugin operation.
	pm.mu.Lock()
	instance, err := pm.getPluginInstanceUnsafe(serverID, instanceID)
	if err != nil {
		pm.mu.Unlock()
		return err
	}

	if instance.Enabled {
		pm.mu.Unlock()
		return nil // Already enabled
	}

	instance.Enabled = true
	instance.UpdatedAt = time.Now()
	needsInit := instance.Status == PluginStatusDisabled
	pm.mu.Unlock()

	if needsInit {
		if err := pm.initializePluginInstance(instance); err != nil {
			// Roll the Enabled flag back so the operator's next attempt is a
			// clean enable rather than an enable-of-already-enabled no-op.
			pm.mu.Lock()
			instance.Enabled = false
			pm.mu.Unlock()
			return fmt.Errorf("failed to initialize plugin instance: %w", err)
		}
	}

	// Persist the enable. Take the lock briefly so the DB write is
	// consistent with any concurrent UpdatedAt bumps.
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if err := pm.updatePluginInstanceInDatabase(instance); err != nil {
		return fmt.Errorf("failed to update plugin instance in database: %w", err)
	}

	return nil
}

// DisablePluginInstance disables a plugin instance
func (pm *PluginManager) DisablePluginInstance(serverID, instanceID uuid.UUID) error {
	// Snapshot the instance under pm.mu, then release before stopPluginInstance
	// which can take up to 30s on a misbehaving plugin. Holding pm.mu across
	// that call would block every other plugin operation.
	pm.mu.Lock()
	instance, err := pm.getPluginInstanceUnsafe(serverID, instanceID)
	if err != nil {
		pm.mu.Unlock()
		return err
	}

	if !instance.Enabled {
		pm.mu.Unlock()
		return nil // Already disabled
	}
	pm.mu.Unlock()

	// Stop plugin (may RPC into subprocess; performed without pm.mu held).
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

	// Re-acquire the lock to commit the disable. If the instance was deleted
	// or replaced while we were doing RPC, treat it as already-gone.
	pm.mu.Lock()
	defer pm.mu.Unlock()

	current, err := pm.getPluginInstanceUnsafe(serverID, instanceID)
	if err != nil || current != instance {
		// Instance is gone or was replaced; nothing more to do.
		return nil
	}

	instance.Enabled = false
	instance.setStatus(PluginStatusDisabled)
	instance.UpdatedAt = time.Now()

	if err := pm.updatePluginInstanceInDatabase(instance); err != nil {
		return fmt.Errorf("failed to update plugin instance in database: %w", err)
	}

	return nil
}

// getDiscordAPI returns the Discord connector API for use by plugins when available.
//
// connectorMu.RLock is held across instance.Connector.GetAPI() because that
// call is in-process for the only Discord connector implementation (it returns
// a cached *discord.DiscordAPI pointer); subprocess-isolated connectors return
// nil from GetAPI and route through the JSON Invoke surface instead, so this
// path can never RPC.
func (pm *PluginManager) getDiscordAPI() DiscordAPI {
	storageKey, ok := pm.ResolveConnectorInstanceKey("com.squad-aegis.connectors.discord")
	if !ok {
		return nil
	}

	pm.connectorMu.RLock()
	instance := pm.connectors[storageKey]
	pm.connectorMu.RUnlock()

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

// instanceDiscordAPI wraps a DiscordAPI with a per-instance channel
// allowlist sourced from the operator-managed instance config key
// `_aegis_discord_channels`. The wrapper exposes allowedDiscordChannels()
// so the host-side dispatcher can enforce the allowlist before invoking
// the underlying connector API.
type instanceDiscordAPI struct {
	DiscordAPI
	pm         *PluginManager
	serverID   uuid.UUID
	instanceID uuid.UUID
}

func (pm *PluginManager) wrapDiscordAPIWithAllowlist(inner DiscordAPI, serverID, instanceID uuid.UUID) DiscordAPI {
	if inner == nil {
		return nil
	}
	return &instanceDiscordAPI{DiscordAPI: inner, pm: pm, serverID: serverID, instanceID: instanceID}
}

func (w *instanceDiscordAPI) allowedDiscordChannels() []string {
	if w == nil || w.pm == nil {
		return nil
	}
	w.pm.mu.RLock()
	defer w.pm.mu.RUnlock()
	serverPlugins, ok := w.pm.plugins[w.serverID]
	if !ok {
		return nil
	}
	instance, ok := serverPlugins[w.instanceID]
	if !ok || instance == nil {
		return nil
	}
	raw, ok := instance.Config["_aegis_discord_channels"]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, s := range v {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	default:
		return nil
	}
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
	type dependent struct {
		serverID   uuid.UUID
		instanceID uuid.UUID
		instance   *PluginInstance
	}

	// Snapshot dependents under the registry lock, then release it before
	// performing N×30s plugin restart RPCs. Holding pm.mu across the restart
	// loop would block every other plugin operation for the duration.
	pm.mu.Lock()
	var dependents []dependent
	for serverID, serverPlugins := range pm.plugins {
		for instanceID, instance := range serverPlugins {
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
			if !pm.pluginDependsOnConnectorRef(definition, connectorID) {
				continue
			}
			if !instance.Enabled || instance.Status != PluginStatusRunning {
				continue
			}
			dependents = append(dependents, dependent{serverID: serverID, instanceID: instanceID, instance: instance})
		}
	}
	pm.mu.Unlock()

	var restartErrors []error
	for _, d := range dependents {
		log.Info().
			Str("serverID", d.serverID.String()).
			Str("instanceID", d.instanceID.String()).
			Str("pluginID", d.instance.PluginID).
			Str("connectorID", connectorID).
			Msg("Restarting plugin due to connector update")

		if err := pm.stopPluginInstance(d.instance); err != nil {
			log.Error().
				Str("serverID", d.serverID.String()).
				Str("instanceID", d.instanceID.String()).
				Str("pluginID", d.instance.PluginID).
				Str("connectorID", connectorID).
				Err(err).
				Msg("Failed to stop plugin instance during connector update restart")
			restartErrors = append(restartErrors, fmt.Errorf("failed to stop plugin %s: %w", d.instanceID.String(), err))
			continue
		}

		if err := pm.initializePluginInstance(d.instance); err != nil {
			log.Error().
				Str("serverID", d.serverID.String()).
				Str("instanceID", d.instanceID.String()).
				Str("pluginID", d.instance.PluginID).
				Str("connectorID", connectorID).
				Err(err).
				Msg("Failed to restart plugin instance after connector update")
			restartErrors = append(restartErrors, fmt.Errorf("failed to restart plugin %s: %w", d.instanceID.String(), err))
			continue
		}

		log.Info().
			Str("serverID", d.serverID.String()).
			Str("instanceID", d.instanceID.String()).
			Str("pluginID", d.instance.PluginID).
			Str("connectorID", connectorID).
			Msg("Successfully restarted plugin after connector update")
	}

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

// initializePluginInstance ensures the plugin's in-process runtime is wired
// up and then drives Initialize/Start RPCs. Callers running outside pm.mu
// should still ensure runtime is created in a serialized way: this function
// performs ensurePluginInstanceRuntime (which may write instance.Plugin) by
// taking pm.mu briefly so concurrent readers under pm.mu.RLock cannot race
// with the write. The RPCs themselves run with no manager-wide lock held so
// a misbehaving subprocess does not stall every other plugin operation.
func (pm *PluginManager) initializePluginInstance(instance *PluginInstance) error {
	// Serialize the (rare) instance.Plugin assignment with concurrent map
	// readers. ensurePluginInstanceRuntime is a pure in-memory operation
	// (registry lookups + struct field writes) so this critical section is
	// short and never blocks on I/O.
	pm.mu.Lock()
	runtimeErr := pm.ensurePluginInstanceRuntime(instance)
	pm.mu.Unlock()
	if runtimeErr != nil {
		instance.setError(PluginStatusError, runtimeErr.Error())
		return runtimeErr
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
		apis.DiscordAPI = pm.wrapDiscordAPIWithAllowlist(pm.getDiscordAPI(), serverID, instanceID)
	}
	if pm.shouldExposePluginAPI(pluginID, NativePluginCapabilityAPILog) {
		apis.LogAPI = NewLogAPI(serverID, instanceID, pluginName, pluginID, logLevel, pm.clickhouseClient, pm.db, pm.eventManager)
	}
	if pm.shouldExposeConnectorAPI(pluginID) {
		apis.ConnectorAPI = newConnectorAPI(pm, pluginID)
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

	// Snapshot the set of instances that handle this event under pm.mu, then
	// release the lock before dispatching. instance.Plugin.GetDefinition() is
	// cached for native plugins and a plain struct return for bundled plugins,
	// so it does not block on I/O — but holding pm.mu across the dispatch goroutine
	// spawn is unnecessary and would extend the read-lock lifetime under load.
	type dispatch struct {
		instance *PluginInstance
	}
	var targets []dispatch

	pm.mu.RLock()
	if serverPlugins, exists := pm.plugins[event.ServerID]; exists {
		for _, instance := range serverPlugins {
			if instance.Status != PluginStatusRunning || !instance.Enabled || instance.Plugin == nil {
				continue
			}

			definition := instance.Plugin.GetDefinition()
			handles := false
			for _, e := range definition.Events {
				if e == event.Type || e == event_manager.EventTypeAll {
					handles = true
					break
				}
			}

			if handles {
				targets = append(targets, dispatch{instance: instance})
			}
		}
	}
	pm.mu.RUnlock()

	for _, t := range targets {
		go pm.handlePluginEvent(t.instance, pluginEvent)
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

		// Get plugin information. instance.Plugin.GetDefinition() is safe to
		// call under pm.mu.RLock here: native shims serve it from a cached
		// field captured at peek time and bundled plugins return a plain
		// struct literal, so no I/O happens while the read lock is held.
		pm.mu.RLock()
		if serverPlugins, exists := pm.plugins[serverID]; exists {
			if instance, exists := serverPlugins[logItem.PluginInstanceID]; exists {
				if instance.Plugin != nil {
					definition := instance.Plugin.GetDefinition()
					logItem.PluginName = definition.Name
					logItem.PluginID = definition.ID
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
	// Snapshot the plugin pointer under pm.mu, then release before
	// GetCommands which RPCs into the subprocess for native plugins.
	// Holding pm.mu across that call would block every other plugin
	// operation.
	pm.mu.RLock()
	instance, err := pm.getPluginInstanceUnsafe(serverID, instanceID)
	if err != nil {
		pm.mu.RUnlock()
		return nil, err
	}
	plugin := instance.Plugin
	pm.mu.RUnlock()

	if plugin == nil {
		return nil, fmt.Errorf("plugin instance not initialized")
	}

	commands := plugin.GetCommands()
	return commands, nil
}

// ExecutePluginCommand executes a command on a plugin instance
func (pm *PluginManager) ExecutePluginCommand(serverID, instanceID uuid.UUID, commandID string, params map[string]interface{}) (*CommandResult, error) {
	// Snapshot the plugin pointer + status under pm.mu, then release the
	// lock before any subprocess RPC. GetCommands and ExecuteCommand both
	// RPC into the subprocess on native plugins, so holding pm.mu would
	// block every other plugin operation for the duration.
	pm.mu.RLock()
	instance, err := pm.getPluginInstanceUnsafe(serverID, instanceID)
	if err != nil {
		pm.mu.RUnlock()
		return nil, err
	}
	plugin := instance.Plugin
	statusAtSnapshot := instance.Status
	enabledAtSnapshot := instance.Enabled
	pluginID := instance.PluginID
	pm.mu.RUnlock()

	if plugin == nil {
		return nil, fmt.Errorf("plugin instance not initialized")
	}

	if statusAtSnapshot != PluginStatusRunning || !enabledAtSnapshot {
		return nil, fmt.Errorf("plugin instance is not running")
	}

	// Validate command exists
	commands := plugin.GetCommands()
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
					Str("pluginID", pluginID).
					Str("commandID", commandID).
					Interface("panic", r).
					Msg("Plugin panicked while executing command")

				execErr = fmt.Errorf("plugin panicked: %v", r)
			}
		}()

		result, execErr = plugin.ExecuteCommand(commandID, params)
	}()

	if execErr != nil {
		return nil, execErr
	}

	return result, nil
}

// GetCommandExecutionStatus gets the status of an async command execution
func (pm *PluginManager) GetCommandExecutionStatus(serverID, instanceID uuid.UUID, executionID string) (*CommandExecutionStatus, error) {
	// Snapshot the plugin pointer under pm.mu, then release before the
	// GetCommandExecutionStatus RPC. Holding pm.mu across that call would
	// block every other plugin operation.
	pm.mu.RLock()
	instance, err := pm.getPluginInstanceUnsafe(serverID, instanceID)
	if err != nil {
		pm.mu.RUnlock()
		return nil, err
	}
	plugin := instance.Plugin
	pm.mu.RUnlock()

	if plugin == nil {
		return nil, fmt.Errorf("plugin instance not initialized")
	}

	status, err := plugin.GetCommandExecutionStatus(executionID)
	if err != nil {
		return nil, err
	}

	return status, nil
}
