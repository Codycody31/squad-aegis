package plugin_loader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/plugin_sdk"
)

// LoadedPlugin represents a loaded .so plugin
type LoadedPlugin struct {
	ID           string
	FilePath     string
	Plugin       *plugin.Plugin
	SDK          plugin_sdk.PluginSDK
	Manifest     plugin_sdk.PluginManifest
	LoadedAt     time.Time
	Verified     bool
	PluginSource PluginSource
}

// PluginSource indicates where the plugin came from
type PluginSource string

const (
	PluginSourceBuiltin PluginSource = "builtin"
	PluginSourceCustom  PluginSource = "custom"
)

// PluginLoader manages loading and unloading of .so plugins
type PluginLoader struct {
	loadedPlugins map[string]*LoadedPlugin
	mu            sync.RWMutex
	tempDir       string
	verifier      *SignatureVerifier
}

// NewPluginLoader creates a new plugin loader
func NewPluginLoader(tempDir string, verifier *SignatureVerifier) (*PluginLoader, error) {
	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	
	return &PluginLoader{
		loadedPlugins: make(map[string]*LoadedPlugin),
		tempDir:       tempDir,
		verifier:      verifier,
	}, nil
}

// LoadPlugin loads a plugin from the specified path
func (pl *PluginLoader) LoadPlugin(ctx context.Context, pluginPath string) (*LoadedPlugin, error) {
	log.Info().Str("path", pluginPath).Msg("Loading plugin from .so file")
	
	// Check if file exists
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("plugin file does not exist: %s", pluginPath)
	}
	
	// Load the plugin
	p, err := plugin.Open(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin: %w", err)
	}
	
	// Look up the PluginExport symbol
	symbol, err := p.Lookup("PluginExport")
	if err != nil {
		return nil, fmt.Errorf("plugin does not export 'PluginExport' symbol: %w", err)
	}
	
	// Assert that PluginExport implements PluginSDK interface
	pluginSDK, ok := symbol.(plugin_sdk.PluginSDK)
	if !ok {
		return nil, fmt.Errorf("PluginExport does not implement plugin_sdk.PluginSDK interface")
	}
	
	// Get the manifest
	manifest := pluginSDK.GetManifest()
	
	// Validate manifest
	if err := plugin_sdk.ValidateManifest(manifest); err != nil {
		return nil, fmt.Errorf("invalid plugin manifest: %w", err)
	}
	
	// Create loaded plugin record
	loadedPlugin := &LoadedPlugin{
		ID:           manifest.ID,
		FilePath:     pluginPath,
		Plugin:       p,
		SDK:          pluginSDK,
		Manifest:     manifest,
		LoadedAt:     time.Now(),
		Verified:     false,
		PluginSource: PluginSourceCustom,
	}
	
	// Store in map
	pl.mu.Lock()
	pl.loadedPlugins[manifest.ID] = loadedPlugin
	pl.mu.Unlock()
	
	log.Info().
		Str("plugin_id", manifest.ID).
		Str("plugin_name", manifest.Name).
		Str("version", manifest.Version).
		Msg("Successfully loaded plugin")
	
	return loadedPlugin, nil
}

// ValidatePlugin validates a plugin's signature and features
func (pl *PluginLoader) ValidatePlugin(ctx context.Context, loadedPlugin *LoadedPlugin, pluginBytes []byte) error {
	manifest := loadedPlugin.Manifest
	
	// Verify signature if present
	if len(manifest.Signature) > 0 {
		if pl.verifier == nil {
			return fmt.Errorf("signature verification is required but no verifier is configured")
		}
		
		if err := pl.verifier.VerifySignature(pluginBytes, manifest.Signature); err != nil {
			return fmt.Errorf("signature verification failed: %w", err)
		}
		
		log.Info().Str("plugin_id", manifest.ID).Msg("Plugin signature verified successfully")
		loadedPlugin.Verified = true
	} else {
		log.Warn().Str("plugin_id", manifest.ID).Msg("Plugin has no signature - running unverified")
	}
	
	// Validate that all required features are supported
	supportedFeatures := GetSupportedFeatures()
	for _, requiredFeature := range manifest.RequiredFeatures {
		if !isFeatureSupported(requiredFeature, supportedFeatures) {
			return fmt.Errorf("plugin requires unsupported feature: %s", requiredFeature)
		}
	}
	
	log.Info().
		Str("plugin_id", manifest.ID).
		Int("required_features", len(manifest.RequiredFeatures)).
		Msg("Plugin feature requirements validated")
	
	return nil
}

// UnloadPlugin marks a plugin as inactive and releases resources
// Note: Go plugins cannot be truly unloaded from memory due to Go's plugin system limitations
func (pl *PluginLoader) UnloadPlugin(pluginID string) error {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	
	loadedPlugin, exists := pl.loadedPlugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin %s is not loaded", pluginID)
	}
	
	// Call the plugin's Shutdown method
	if err := loadedPlugin.SDK.Shutdown(); err != nil {
		log.Error().
			Err(err).
			Str("plugin_id", pluginID).
			Msg("Error during plugin shutdown")
		return fmt.Errorf("failed to shutdown plugin: %w", err)
	}
	
	// Remove from map (but cannot unload from memory)
	delete(pl.loadedPlugins, pluginID)
	
	log.Info().Str("plugin_id", pluginID).Msg("Plugin unloaded (marked inactive)")
	
	// Note: The .so file remains loaded in memory due to Go plugin limitations
	// This is a known limitation of Go's plugin system
	
	return nil
}

// GetLoadedPlugin returns a loaded plugin by ID
func (pl *PluginLoader) GetLoadedPlugin(pluginID string) (*LoadedPlugin, error) {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	
	loadedPlugin, exists := pl.loadedPlugins[pluginID]
	if !exists {
		return nil, fmt.Errorf("plugin %s is not loaded", pluginID)
	}
	
	return loadedPlugin, nil
}

// ListLoadedPlugins returns all currently loaded plugins
func (pl *PluginLoader) ListLoadedPlugins() []*LoadedPlugin {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	
	plugins := make([]*LoadedPlugin, 0, len(pl.loadedPlugins))
	for _, p := range pl.loadedPlugins {
		plugins = append(plugins, p)
	}
	
	return plugins
}

// GetTempPluginPath returns a path in the temp directory for a plugin
func (pl *PluginLoader) GetTempPluginPath(pluginID, version string) string {
	filename := fmt.Sprintf("%s_%s_%s.so", pluginID, version, uuid.New().String()[:8])
	return filepath.Join(pl.tempDir, filename)
}

// Cleanup removes temporary plugin files
func (pl *PluginLoader) Cleanup() error {
	log.Info().Str("temp_dir", pl.tempDir).Msg("Cleaning up temporary plugin files")
	
	// Read directory
	entries, err := os.ReadDir(pl.tempDir)
	if err != nil {
		return fmt.Errorf("failed to read temp directory: %w", err)
	}
	
	// Remove .so files
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".so" {
			path := filepath.Join(pl.tempDir, entry.Name())
			if err := os.Remove(path); err != nil {
				log.Error().Err(err).Str("file", path).Msg("Failed to remove temp plugin file")
			}
		}
	}
	
	return nil
}

// GetSupportedFeatures returns the list of features supported by the current API version
func GetSupportedFeatures() []plugin_sdk.FeatureID {
	return []plugin_sdk.FeatureID{
		plugin_sdk.FeatureEventHandling,
		plugin_sdk.FeatureRCON,
		plugin_sdk.FeatureDatabaseAccess,
		plugin_sdk.FeatureCommands,
		plugin_sdk.FeatureConnectors,
		plugin_sdk.FeatureAdminAPI,
		plugin_sdk.FeatureServerAPI,
		plugin_sdk.FeatureLogging,
		// Future features can be added here as they're implemented
	}
}

// isFeatureSupported checks if a feature is in the supported list
func isFeatureSupported(feature plugin_sdk.FeatureID, supportedFeatures []plugin_sdk.FeatureID) bool {
	for _, supported := range supportedFeatures {
		if feature == supported {
			return true
		}
	}
	return false
}

// IsPluginLoaded checks if a plugin is currently loaded
func (pl *PluginLoader) IsPluginLoaded(pluginID string) bool {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	_, exists := pl.loadedPlugins[pluginID]
	return exists
}

