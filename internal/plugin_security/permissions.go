package plugin_security

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/plugin_sdk"
)

// PermissionManager manages plugin permissions
type PermissionManager struct {
	db          *sql.DB
	permissions map[string]map[plugin_sdk.PermissionID]bool // pluginID -> permission -> granted
	mu          sync.RWMutex
}

// NewPermissionManager creates a new permission manager
func NewPermissionManager(db *sql.DB) (*PermissionManager, error) {
	pm := &PermissionManager{
		db:          db,
		permissions: make(map[string]map[plugin_sdk.PermissionID]bool),
	}
	
	// Load permissions from database
	if err := pm.loadPermissions(); err != nil {
		return nil, fmt.Errorf("failed to load permissions: %w", err)
	}
	
	return pm, nil
}

// CheckPermission checks if a plugin has a specific permission
func (pm *PermissionManager) CheckPermission(pluginID string, permission plugin_sdk.PermissionID) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	pluginPerms, exists := pm.permissions[pluginID]
	if !exists {
		return false
	}
	
	return pluginPerms[permission]
}

// GrantPermission grants a permission to a plugin
func (pm *PermissionManager) GrantPermission(pluginID string, permission plugin_sdk.PermissionID, grantedBy uuid.UUID) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// Check if permission already granted
	if pluginPerms, exists := pm.permissions[pluginID]; exists {
		if pluginPerms[permission] {
			return nil // Already granted
		}
	}
	
	// Insert into database
	query := `
		INSERT INTO plugin_permissions (id, plugin_id, permission_id, granted_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (plugin_id, permission_id) DO NOTHING
	`
	
	if _, err := pm.db.Exec(query, uuid.New(), pluginID, string(permission), grantedBy); err != nil {
		return fmt.Errorf("failed to grant permission: %w", err)
	}
	
	// Update in memory
	if pm.permissions[pluginID] == nil {
		pm.permissions[pluginID] = make(map[plugin_sdk.PermissionID]bool)
	}
	pm.permissions[pluginID][permission] = true
	
	log.Info().
		Str("plugin_id", pluginID).
		Str("permission", string(permission)).
		Str("granted_by", grantedBy.String()).
		Msg("Permission granted to plugin")
	
	return nil
}

// RevokePermission revokes a permission from a plugin
func (pm *PermissionManager) RevokePermission(pluginID string, permission plugin_sdk.PermissionID) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// Delete from database
	query := `DELETE FROM plugin_permissions WHERE plugin_id = $1 AND permission_id = $2`
	
	if _, err := pm.db.Exec(query, pluginID, string(permission)); err != nil {
		return fmt.Errorf("failed to revoke permission: %w", err)
	}
	
	// Update in memory
	if pluginPerms, exists := pm.permissions[pluginID]; exists {
		delete(pluginPerms, permission)
	}
	
	log.Info().
		Str("plugin_id", pluginID).
		Str("permission", string(permission)).
		Msg("Permission revoked from plugin")
	
	return nil
}

// GrantPermissions grants multiple permissions to a plugin
func (pm *PermissionManager) GrantPermissions(pluginID string, permissions []plugin_sdk.PermissionID, grantedBy uuid.UUID) error {
	for _, perm := range permissions {
		if err := pm.GrantPermission(pluginID, perm, grantedBy); err != nil {
			return fmt.Errorf("failed to grant permission %s: %w", perm, err)
		}
	}
	return nil
}

// GetPluginPermissions returns all permissions granted to a plugin
func (pm *PermissionManager) GetPluginPermissions(pluginID string) []plugin_sdk.PermissionID {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	pluginPerms, exists := pm.permissions[pluginID]
	if !exists {
		return []plugin_sdk.PermissionID{}
	}
	
	perms := make([]plugin_sdk.PermissionID, 0, len(pluginPerms))
	for perm, granted := range pluginPerms {
		if granted {
			perms = append(perms, perm)
		}
	}
	
	return perms
}

// RevokeAllPermissions revokes all permissions from a plugin
func (pm *PermissionManager) RevokeAllPermissions(pluginID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// Delete from database
	query := `DELETE FROM plugin_permissions WHERE plugin_id = $1`
	
	if _, err := pm.db.Exec(query, pluginID); err != nil {
		return fmt.Errorf("failed to revoke all permissions: %w", err)
	}
	
	// Update in memory
	delete(pm.permissions, pluginID)
	
	log.Info().
		Str("plugin_id", pluginID).
		Msg("All permissions revoked from plugin")
	
	return nil
}

// loadPermissions loads all permissions from the database
func (pm *PermissionManager) loadPermissions() error {
	query := `
		SELECT plugin_id, permission_id
		FROM plugin_permissions
		ORDER BY granted_at ASC
	`
	
	rows, err := pm.db.Query(query)
	if err != nil {
		// If table doesn't exist yet, that's okay
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("failed to query permissions: %w", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		var pluginID string
		var permissionID string
		
		if err := rows.Scan(&pluginID, &permissionID); err != nil {
			return fmt.Errorf("failed to scan permission row: %w", err)
		}
		
		if pm.permissions[pluginID] == nil {
			pm.permissions[pluginID] = make(map[plugin_sdk.PermissionID]bool)
		}
		pm.permissions[pluginID][plugin_sdk.PermissionID(permissionID)] = true
	}
	
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating permission rows: %w", err)
	}
	
	log.Info().Int("plugin_count", len(pm.permissions)).Msg("Loaded plugin permissions")
	
	return nil
}

// ReloadPermissions reloads permissions from the database
func (pm *PermissionManager) ReloadPermissions() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.permissions = make(map[string]map[plugin_sdk.PermissionID]bool)
	return pm.loadPermissions()
}

// ValidateRequiredPermissions checks if a plugin has all its required permissions
func (pm *PermissionManager) ValidateRequiredPermissions(pluginID string, requiredPermissions []plugin_sdk.PermissionID) error {
	for _, perm := range requiredPermissions {
		if !pm.CheckPermission(pluginID, perm) {
			return fmt.Errorf("plugin %s is missing required permission: %s", pluginID, perm)
		}
	}
	return nil
}

// PermissionInfo contains information about a granted permission
type PermissionInfo struct {
	PluginID     string                `json:"plugin_id"`
	PermissionID plugin_sdk.PermissionID `json:"permission_id"`
	GrantedBy    uuid.UUID             `json:"granted_by"`
	GrantedAt    string                `json:"granted_at"`
}

// ListAllPermissions returns all permissions in the system
func (pm *PermissionManager) ListAllPermissions() ([]PermissionInfo, error) {
	query := `
		SELECT plugin_id, permission_id, granted_by, granted_at
		FROM plugin_permissions
		ORDER BY granted_at DESC
	`
	
	rows, err := pm.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query permissions: %w", err)
	}
	defer rows.Close()
	
	var permissions []PermissionInfo
	for rows.Next() {
		var info PermissionInfo
		var permID string
		
		if err := rows.Scan(&info.PluginID, &permID, &info.GrantedBy, &info.GrantedAt); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		
		info.PermissionID = plugin_sdk.PermissionID(permID)
		permissions = append(permissions, info)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating permissions: %w", err)
	}
	
	return permissions, nil
}

// GetAllDefinedPermissions returns all permission IDs that can be granted
func GetAllDefinedPermissions() []plugin_sdk.PermissionID {
	return []plugin_sdk.PermissionID{
		plugin_sdk.PermissionRCONAccess,
		plugin_sdk.PermissionRCONBroadcast,
		plugin_sdk.PermissionRCONKick,
		plugin_sdk.PermissionRCONBan,
		plugin_sdk.PermissionRCONAdmin,
		plugin_sdk.PermissionDatabaseRead,
		plugin_sdk.PermissionDatabaseWrite,
		plugin_sdk.PermissionAdminManagement,
		plugin_sdk.PermissionPlayerManagement,
		plugin_sdk.PermissionEventPublish,
		plugin_sdk.PermissionConnectorAccess,
	}
}

// GetPermissionDescription returns a human-readable description of a permission
func GetPermissionDescription(permission plugin_sdk.PermissionID) string {
	descriptions := map[plugin_sdk.PermissionID]string{
		plugin_sdk.PermissionRCONAccess:       "Access to RCON commands",
		plugin_sdk.PermissionRCONBroadcast:    "Broadcast messages to all players",
		plugin_sdk.PermissionRCONKick:         "Kick players from the server",
		plugin_sdk.PermissionRCONBan:          "Ban players from the server",
		plugin_sdk.PermissionRCONAdmin:        "Manage admin lists via RCON",
		plugin_sdk.PermissionDatabaseRead:     "Read data from the database",
		plugin_sdk.PermissionDatabaseWrite:    "Write data to the database",
		plugin_sdk.PermissionAdminManagement:  "Manage server administrators",
		plugin_sdk.PermissionPlayerManagement: "Manage player data and actions",
		plugin_sdk.PermissionEventPublish:     "Publish events to the system",
		plugin_sdk.PermissionConnectorAccess:  "Access external connectors (Discord, etc.)",
	}
	
	if desc, exists := descriptions[permission]; exists {
		return desc
	}
	
	return "Unknown permission"
}

