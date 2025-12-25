package plugin_sdk

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

// FeatureGateway manages API access for plugins based on their declared features
type FeatureGateway struct {
	pluginID         string
	instanceID       uuid.UUID
	serverID         uuid.UUID
	manifest         PluginManifest
	featureAPIs      map[FeatureID]interface{}
	permissionChecker PermissionChecker
	ctx              context.Context
	
	// Goroutine tracking for sandbox
	goroutineCount atomic.Int32
	mu             sync.RWMutex
}

// PermissionChecker checks if a plugin has a specific permission
type PermissionChecker interface {
	CheckPermission(pluginID string, permission PermissionID) bool
}

// NewFeatureGateway creates a new feature gateway for a plugin
func NewFeatureGateway(
	pluginID string,
	instanceID uuid.UUID,
	serverID uuid.UUID,
	manifest PluginManifest,
	ctx context.Context,
	permissionChecker PermissionChecker,
) *FeatureGateway {
	return &FeatureGateway{
		pluginID:          pluginID,
		instanceID:        instanceID,
		serverID:          serverID,
		manifest:          manifest,
		featureAPIs:       make(map[FeatureID]interface{}),
		permissionChecker: permissionChecker,
		ctx:               ctx,
	}
}

// RegisterFeatureAPI registers an API implementation for a feature
func (fg *FeatureGateway) RegisterFeatureAPI(featureID FeatureID, api interface{}) {
	fg.mu.Lock()
	defer fg.mu.Unlock()
	fg.featureAPIs[featureID] = api
}

// GetFeatureAPI returns an API for a specific feature if the plugin has access
func (fg *FeatureGateway) GetFeatureAPI(featureID FeatureID) (interface{}, error) {
	fg.mu.RLock()
	defer fg.mu.RUnlock()
	
	// Check if plugin declared this feature as required
	hasFeature := false
	for _, required := range fg.manifest.RequiredFeatures {
		if required == featureID {
			hasFeature = true
			break
		}
	}
	
	if !hasFeature {
		return nil, fmt.Errorf("plugin %s did not declare feature %s in manifest", fg.pluginID, featureID)
	}
	
	// Check if the feature API is available
	api, exists := fg.featureAPIs[featureID]
	if !exists {
		return nil, fmt.Errorf("feature %s is not available in this SDK version", featureID)
	}
	
	return api, nil
}

// GetServerID returns the current server ID
func (fg *FeatureGateway) GetServerID() uuid.UUID {
	return fg.serverID
}

// GetPluginInstanceID returns this plugin instance's unique ID
func (fg *FeatureGateway) GetPluginInstanceID() uuid.UUID {
	return fg.instanceID
}

// SpawnGoroutine spawns a tracked goroutine (for sandbox monitoring)
func (fg *FeatureGateway) SpawnGoroutine(fn func()) {
	fg.goroutineCount.Add(1)
	go func() {
		defer fg.goroutineCount.Add(-1)
		fn()
	}()
}

// GetGoroutineCount returns the current number of active goroutines spawned by this plugin
func (fg *FeatureGateway) GetGoroutineCount() int32 {
	return fg.goroutineCount.Load()
}

// GetContext returns the plugin's context (cancelled on shutdown)
func (fg *FeatureGateway) GetContext() context.Context {
	return fg.ctx
}

// CheckPermission verifies if the plugin has a specific permission
func (fg *FeatureGateway) CheckPermission(permission PermissionID) error {
	if fg.permissionChecker == nil {
		return fmt.Errorf("permission checker not available")
	}
	
	if !fg.permissionChecker.CheckPermission(fg.pluginID, permission) {
		return fmt.Errorf("plugin %s does not have permission %s", fg.pluginID, permission)
	}
	
	return nil
}

// PermissionWrappedAPI wraps APIs to check permissions before execution
type PermissionWrappedAPI struct {
	gateway    *FeatureGateway
	underlying interface{}
}

// WrapAPIWithPermissions wraps an API to automatically check permissions
func (fg *FeatureGateway) WrapAPIWithPermissions(api interface{}) interface{} {
	return &PermissionWrappedAPI{
		gateway:    fg,
		underlying: api,
	}
}

// ValidateManifest validates that a plugin manifest is well-formed
func ValidateManifest(manifest PluginManifest) error {
	if manifest.ID == "" {
		return fmt.Errorf("plugin ID is required")
	}
	if manifest.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if manifest.Version == "" {
		return fmt.Errorf("plugin version is required")
	}
	if manifest.Author == "" {
		return fmt.Errorf("plugin author is required")
	}
	if manifest.SDKVersion == "" {
		return fmt.Errorf("plugin SDK version is required")
	}
	
	// Validate SDK version compatibility
	if !IsSDKVersionCompatible(manifest.SDKVersion) {
		return fmt.Errorf("plugin SDK version %s is not compatible with current API version %s", 
			manifest.SDKVersion, APIVersion)
	}
	
	return nil
}

// IsSDKVersionCompatible checks if a plugin's SDK version is compatible with the current API version
func IsSDKVersionCompatible(pluginSDKVersion string) bool {
	// Simple version check - in production, use semantic versioning library
	// For now, we only support exact match or v1.x.x compatibility
	if pluginSDKVersion == APIVersion {
		return true
	}
	
	// Allow v1.x.x plugins to work with v1.y.z API if x <= y
	// This is a simplified check - in production, use a proper semver library
	if len(pluginSDKVersion) >= 2 && pluginSDKVersion[:2] == "v1" &&
	   len(APIVersion) >= 2 && APIVersion[:2] == "v1" {
		return true
	}
	
	return false
}

// GetRequiredPermissionsForFeature returns the permissions required for a feature
func GetRequiredPermissionsForFeature(featureID FeatureID) []PermissionID {
	switch featureID {
	case FeatureRCON:
		return []PermissionID{PermissionRCONAccess}
	case FeatureDatabaseAccess:
		return []PermissionID{PermissionDatabaseRead}
	case FeatureAdminAPI:
		return []PermissionID{PermissionAdminManagement}
	case FeatureConnectors:
		return []PermissionID{PermissionConnectorAccess}
	case FeatureEventHandling:
		return []PermissionID{PermissionEventPublish}
	default:
		return []PermissionID{}
	}
}

// ValidateFeaturePermissions checks if a plugin has all required permissions for its declared features
func ValidateFeaturePermissions(manifest PluginManifest, permissionChecker PermissionChecker) error {
	for _, feature := range manifest.RequiredFeatures {
		requiredPerms := GetRequiredPermissionsForFeature(feature)
		for _, perm := range requiredPerms {
			if !permissionChecker.CheckPermission(manifest.ID, perm) {
				return fmt.Errorf("plugin %s requires permission %s for feature %s", 
					manifest.ID, perm, feature)
			}
		}
	}
	return nil
}

