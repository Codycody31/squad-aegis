package permissions

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Service provides permission checking with caching.
type Service struct {
	db    *sql.DB
	cache *Cache
}

// Cache stores permission data with TTL.
type Cache struct {
	mu              sync.RWMutex
	userPermissions map[string]cacheEntry // key: "userId:serverId" -> permissions
	ttl             time.Duration
}

type cacheEntry struct {
	permissions []Permission
	expiry      time.Time
}

// NewService creates a new permission service.
func NewService(db *sql.DB) *Service {
	return &Service{
		db: db,
		cache: &Cache{
			userPermissions: make(map[string]cacheEntry),
			ttl:             5 * time.Minute,
		},
	}
}

// HasPermission checks if a user has a specific permission for a server.
func (s *Service) HasPermission(ctx context.Context, userId, serverId uuid.UUID, permission Permission) (bool, error) {
	permissions, err := s.GetUserServerPermissions(ctx, userId, serverId)
	if err != nil {
		return false, err
	}

	return EvaluatePermission(permissions, permission), nil
}

// HasAnyPermission checks if a user has any of the specified permissions.
func (s *Service) HasAnyPermission(ctx context.Context, userId, serverId uuid.UUID, perms ...Permission) (bool, error) {
	userPerms, err := s.GetUserServerPermissions(ctx, userId, serverId)
	if err != nil {
		return false, err
	}

	for _, perm := range perms {
		if EvaluatePermission(userPerms, perm) {
			return true, nil
		}
	}
	return false, nil
}

// HasAllPermissions checks if a user has all specified permissions.
func (s *Service) HasAllPermissions(ctx context.Context, userId, serverId uuid.UUID, perms ...Permission) (bool, error) {
	userPerms, err := s.GetUserServerPermissions(ctx, userId, serverId)
	if err != nil {
		return false, err
	}

	for _, perm := range perms {
		if !EvaluatePermission(userPerms, perm) {
			return false, nil
		}
	}
	return true, nil
}

// GetUserServerPermissions retrieves all permissions for a user on a specific server.
func (s *Service) GetUserServerPermissions(ctx context.Context, userId, serverId uuid.UUID) ([]Permission, error) {
	cacheKey := userId.String() + ":" + serverId.String()

	// Check cache first
	if cached := s.cache.get(cacheKey); cached != nil {
		return cached, nil
	}

	// Query with role inheritance support
	query := `
		WITH RECURSIVE role_hierarchy AS (
			-- Base: Get user's direct role
			SELECT sr.id as role_id, 0 as depth
			FROM server_admins sa
			JOIN server_roles sr ON sa.server_role_id = sr.id
			WHERE sa.user_id = $1 AND sa.server_id = $2
			AND (sa.expires_at IS NULL OR sa.expires_at > NOW())

			UNION

			-- Recursive: Get inherited roles
			SELECT ri.parent_role_id, rh.depth + 1
			FROM role_inheritance ri
			JOIN role_hierarchy rh ON ri.child_role_id = rh.role_id
			WHERE rh.depth < 5  -- Prevent infinite loops
		)
		SELECT DISTINCT p.code
		FROM role_hierarchy rh
		JOIN server_role_permissions srp ON rh.role_id = srp.server_role_id
		JOIN permissions p ON srp.permission_id = p.id
	`

	rows, err := s.db.QueryContext(ctx, query, userId, serverId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []Permission
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		permissions = append(permissions, Permission(code))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Cache the result
	s.cache.set(cacheKey, permissions)

	return permissions, nil
}

// GetUserServerPermissionsByIdentifiers retrieves permissions for direct admin assignments
// using any known Steam/EOS identifiers.
func (s *Service) GetUserServerPermissionsByIdentifiers(ctx context.Context, steamId *int64, eosID string, serverId uuid.UUID) ([]Permission, error) {
	eosID = strings.TrimSpace(eosID)

	query := `
		WITH RECURSIVE role_hierarchy AS (
			SELECT sr.id as role_id, 0 as depth
			FROM server_admins sa
			JOIN server_roles sr ON sa.server_role_id = sr.id
			LEFT JOIN users u ON sa.user_id = u.id
			WHERE sa.server_id = $1
			  AND (sa.expires_at IS NULL OR sa.expires_at > NOW())
			  AND (
				($2::bigint IS NOT NULL AND (sa.steam_id = $2::bigint OR u.steam_id = $2::bigint))
				OR ($3::text IS NOT NULL AND sa.eos_id = $3::text)
			  )

			UNION

			SELECT ri.parent_role_id, rh.depth + 1
			FROM role_inheritance ri
			JOIN role_hierarchy rh ON ri.child_role_id = rh.role_id
			WHERE rh.depth < 5
		)
		SELECT DISTINCT p.code
		FROM role_hierarchy rh
		JOIN server_role_permissions srp ON rh.role_id = srp.server_role_id
		JOIN permissions p ON srp.permission_id = p.id
	`

	var steamIDArg interface{}
	if steamId != nil {
		steamIDArg = *steamId
	}

	var eosIDArg interface{}
	if eosID != "" {
		eosIDArg = eosID
	}

	rows, err := s.db.QueryContext(ctx, query, serverId, steamIDArg, eosIDArg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []Permission
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		permissions = append(permissions, Permission(code))
	}

	return permissions, rows.Err()
}

// GetUserServerPermissionsBySteamId retrieves permissions for a Steam ID on a specific server.
// Used for players who may not have a user account.
func (s *Service) GetUserServerPermissionsBySteamId(ctx context.Context, steamId int64, serverId uuid.UUID) ([]Permission, error) {
	return s.GetUserServerPermissionsByIdentifiers(ctx, &steamId, "", serverId)
}

// GetRolePermissions retrieves all permissions for a specific role.
func (s *Service) GetRolePermissions(ctx context.Context, roleId uuid.UUID) ([]Permission, error) {
	query := `
		SELECT p.code
		FROM server_role_permissions srp
		JOIN permissions p ON srp.permission_id = p.id
		WHERE srp.server_role_id = $1
	`

	rows, err := s.db.QueryContext(ctx, query, roleId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []Permission
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		permissions = append(permissions, Permission(code))
	}

	return permissions, rows.Err()
}

// GetRCONPermissionsForExport retrieves RCON permissions for a role in Squad admin.cfg format.
func (s *Service) GetRCONPermissionsForExport(ctx context.Context, roleId uuid.UUID) ([]string, error) {
	perms, err := s.GetRolePermissions(ctx, roleId)
	if err != nil {
		return nil, err
	}

	var squadPerms []string
	for _, p := range perms {
		if p == Wildcard {
			// Wildcard means all RCON permissions
			for _, rconPerm := range RCONPermissions() {
				if squadPerm := rconPerm.ToSquadPermission(); squadPerm != "" {
					squadPerms = append(squadPerms, squadPerm)
				}
			}
			break
		}
		if squadPerm := p.ToSquadPermission(); squadPerm != "" {
			squadPerms = append(squadPerms, squadPerm)
		}
	}

	return squadPerms, nil
}

// InvalidateUserCache clears the permission cache for a user on a specific server.
func (s *Service) InvalidateUserCache(userId, serverId uuid.UUID) {
	cacheKey := userId.String() + ":" + serverId.String()
	s.cache.delete(cacheKey)
}

// InvalidateServerCache clears all permission caches for a server.
func (s *Service) InvalidateServerCache(serverId uuid.UUID) {
	s.cache.deleteByServerPrefix(serverId.String())
}

// InvalidateAllCache clears the entire permission cache.
func (s *Service) InvalidateAllCache() {
	s.cache.clear()
}

// EvaluatePermission checks if a permission is granted based on user's permissions.
func EvaluatePermission(userPerms []Permission, required Permission) bool {
	for _, p := range userPerms {
		// Wildcard grants all
		if p == Wildcard {
			return true
		}
		// Exact match
		if p == required {
			return true
		}
		// Category wildcard (e.g., "ui:*" grants all UI permissions)
		if strings.HasSuffix(string(p), ":*") {
			prefix := strings.TrimSuffix(string(p), "*")
			if strings.HasPrefix(string(required), prefix) {
				return true
			}
		}
	}
	return false
}

// EvaluateAnyPermission checks if any of the required permissions are granted.
func EvaluateAnyPermission(userPerms []Permission, required ...Permission) bool {
	for _, req := range required {
		if EvaluatePermission(userPerms, req) {
			return true
		}
	}
	return false
}

// EvaluateAllPermissions checks if all required permissions are granted.
func EvaluateAllPermissions(userPerms []Permission, required ...Permission) bool {
	for _, req := range required {
		if !EvaluatePermission(userPerms, req) {
			return false
		}
	}
	return true
}

// Cache methods

func (c *Cache) get(key string) []Permission {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.userPermissions[key]
	if !exists || time.Now().After(entry.expiry) {
		return nil
	}
	return entry.permissions
}

func (c *Cache) set(key string, permissions []Permission) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.userPermissions[key] = cacheEntry{
		permissions: permissions,
		expiry:      time.Now().Add(c.ttl),
	}
}

func (c *Cache) delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.userPermissions, key)
}

func (c *Cache) deleteByServerPrefix(serverId string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	suffix := ":" + serverId
	for key := range c.userPermissions {
		if strings.HasSuffix(key, suffix) {
			delete(c.userPermissions, key)
		}
	}
}

func (c *Cache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.userPermissions = make(map[string]cacheEntry)
}
