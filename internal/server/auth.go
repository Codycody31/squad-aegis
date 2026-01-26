package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/leighmacdonald/steamid/v3/steamid"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/permissions"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

type AuthLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UpdateProfileRequest struct {
	Name    string `json:"name" binding:"required"`
	SteamId string `json:"steam_id"`
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=8"`
}

func (s *Server) AuthLogin(c *gin.Context) {
	var req AuthLoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request payload", nil)
		return
	}

	tx, err := s.Dependencies.DB.BeginTx(c.Copy(), nil)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	user, err := core.AuthenticateUser(c.Copy(), tx, req.Username, req.Password)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	session, err := core.CreateSession(c.Copy(), tx, user.Id, c.ClientIP(), time.Hour*24)
	if err != nil {
		fmt.Println(err)
		err := tx.Rollback()
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to rollback transaction: %w", err), nil)
			return
		}
		responses.InternalServerError(c, err, nil)
		return
	}

	err = tx.Commit()
	if err != nil {
		fmt.Println(err)
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "User logged in successfully", &gin.H{
		"session": gin.H{
			"token":      session.Token,
			"expires_at": session.ExpiresAt,
		},
	})
}

func (s *Server) AuthLogout(c *gin.Context) {
	session := c.MustGet("session").(*models.Session)

	tx, err := s.Dependencies.DB.BeginTx(c.Copy(), nil)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	err = core.DeleteSessionById(c.Copy(), tx, session.Id)
	if err != nil {
		err := tx.Rollback()
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to rollback transaction: %w", err), nil)
			return
		}
		responses.InternalServerError(c, err, nil)
		return
	}

	err = tx.Commit()
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.SimpleSuccess(c, "User logged out")
}

func (s *Server) AuthInitial(c *gin.Context) {
	session := c.MustGet("session").(*models.Session)

	user, err := core.GetUserById(c.Copy(), s.Dependencies.DB, session.UserId, &session.UserId)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	// Get user's server permissions using new PBAC system
	serverPermissions, err := s.getUserAllServerPermissions(c, session.UserId)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "User authenticated", &gin.H{
		"user":              user,
		"serverPermissions": serverPermissions,
	})
}

// getUserAllServerPermissions retrieves permissions for all servers a user has access to
func (s *Server) getUserAllServerPermissions(c *gin.Context, userId uuid.UUID) (map[string][]string, error) {
	query := `
		SELECT DISTINCT sa.server_id, p.code
		FROM server_admins sa
		JOIN server_roles sr ON sa.server_role_id = sr.id
		JOIN server_role_permissions srp ON sr.id = srp.server_role_id
		JOIN permissions p ON srp.permission_id = p.id
		WHERE sa.user_id = $1
		AND (sa.expires_at IS NULL OR sa.expires_at > NOW())
		ORDER BY sa.server_id, p.code
	`

	rows, err := s.Dependencies.DB.QueryContext(c, query, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to query permissions: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var serverId uuid.UUID
		var permCode string
		if err := rows.Scan(&serverId, &permCode); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		serverIdStr := serverId.String()
		result[serverIdStr] = append(result[serverIdStr], permCode)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating permissions: %w", err)
	}

	return result, nil
}

func (s *Server) UpdateUserProfile(c *gin.Context) {
	session := c.MustGet("session").(*models.Session)
	var req UpdateProfileRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request payload", nil)
		return
	}

	sid64 := steamid.New(req.SteamId)
	if !sid64.Valid() {
		responses.BadRequest(c, "Invalid Steam ID", nil)
		return
	}

	err := core.UpdateUserProfile(c.Copy(), s.Dependencies.DB, session.UserId, req.Name, int(sid64.Int64()))
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.SimpleSuccess(c, "Profile updated successfully")
}

func (s *Server) UpdateUserPassword(c *gin.Context) {
	session := c.MustGet("session").(*models.Session)
	var req UpdatePasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request payload", nil)
		return
	}

	tx, err := s.Dependencies.DB.BeginTx(c.Copy(), nil)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer tx.Rollback()

	// First verify the current password
	user, err := core.GetUserById(c.Copy(), tx, session.UserId, &session.UserId)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	if err := user.ComparePassword(req.CurrentPassword); err != nil {
		responses.BadRequest(c, "Current password is incorrect", nil)
		return
	}

	// Update the password
	err = core.UpdateUserPassword(c.Copy(), tx, session.UserId, req.NewPassword)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	if err := tx.Commit(); err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.SimpleSuccess(c, "Password updated successfully")
}

// GetUserServerPermissions retrieves the permissions a user has for a specific server
func (s *Server) GetUserServerPermissions(c *gin.Context, userId, serverId uuid.UUID) ([]string, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("sr.permissions").
		From("server_admins sa").
		Join("server_roles sr ON sa.server_role_id = sr.id").
		Where(squirrel.Eq{"sa.server_id": serverId, "sa.user_id": userId}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to create SQL query: %w", err)
	}

	var permissionsStr string
	err = s.Dependencies.DB.QueryRowContext(c, sql, args...).Scan(&permissionsStr)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return []string{}, nil // User has no permissions for this server
		}
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}

	// Parse permissions from comma-separated string
	if permissionsStr == "" {
		return []string{}, nil
	}

	permissions := strings.Split(permissionsStr, ",")
	// Trim whitespace from each permission
	for i, perm := range permissions {
		permissions[i] = strings.TrimSpace(perm)
	}

	return permissions, nil
}

// userHasServerPermission checks if a user has a specific permission for a server
func (s *Server) userHasServerPermission(c *gin.Context, userId, serverId uuid.UUID, requiredPermission string) (bool, error) {
	permissions, err := s.GetUserServerPermissions(c, userId, serverId)
	if err != nil {
		return false, err
	}

	// Check if the user has the required permission
	for _, perm := range permissions {
		if perm == requiredPermission || perm == "*" {
			return true, nil
		}
	}

	return false, nil
}

// userHasAnyServerPermission checks if a user has any of the specified permissions for a server
func (s *Server) userHasAnyServerPermission(c *gin.Context, userId, serverId uuid.UUID, requiredPermissions []string) (bool, error) {
	permissions, err := s.GetUserServerPermissions(c, userId, serverId)
	if err != nil {
		return false, err
	}

	// Check if the user has any of the required permissions
	for _, userPerm := range permissions {
		if userPerm == "*" {
			return true, nil
		}
		for _, requiredPerm := range requiredPermissions {
			if userPerm == requiredPerm {
				return true, nil
			}
		}
	}

	return false, nil
}

// AuthHasServerPermission checks if the user has a specific permission for a server
// This middleware expects the serverId to be in the URL parameters
func (s *Server) AuthHasServerPermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the user from the session
		user := s.getUserFromSession(c)
		if user == nil {
			responses.Unauthorized(c, "Unauthorized", nil)
			c.Abort()
			return
		}

		// Super admins have all permissions
		if user.SuperAdmin {
			c.Next()
			return
		}

		// Get the server ID from the URL parameters
		serverIdString := c.Param("serverId")
		if serverIdString == "" {
			responses.BadRequest(c, "Server ID is required", nil)
			c.Abort()
			return
		}

		serverId, err := uuid.Parse(serverIdString)
		if err != nil {
			responses.BadRequest(c, "Invalid server ID", nil)
			c.Abort()
			return
		}

		// Check if the user has the required permission
		hasPermission, err := s.userHasServerPermission(c.Copy(), user.Id, serverId, permission)
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to check permissions: %w", err), nil)
			c.Abort()
			return
		}

		if !hasPermission {
			responses.Forbidden(c, "You don't have the required permission", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// AuthHasAnyServerPermission checks if the user has any of the specified permissions for a server
// DEPRECATED: Use RequireAnyPermission instead for new code
func (s *Server) AuthHasAnyServerPermission(perms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the user from the session
		user := s.getUserFromSession(c)
		if user == nil {
			responses.Unauthorized(c, "Unauthorized", nil)
			c.Abort()
			return
		}

		// Super admins have all permissions
		if user.SuperAdmin {
			c.Next()
			return
		}

		// Get the server ID from the URL parameters
		serverIdString := c.Param("serverId")
		if serverIdString == "" {
			responses.BadRequest(c, "Server ID is required", nil)
			c.Abort()
			return
		}

		serverId, err := uuid.Parse(serverIdString)
		if err != nil {
			responses.BadRequest(c, "Invalid server ID", nil)
			c.Abort()
			return
		}

		// Check if the user has any of the required permissions
		hasPermission, err := s.userHasAnyServerPermission(c.Copy(), user.Id, serverId, perms)
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to check permissions: %w", err), nil)
			c.Abort()
			return
		}

		if !hasPermission {
			responses.Forbidden(c, "You don't have any of the required permissions", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// =============================================================================
// NEW PBAC Permission Middleware (uses permissions.Permission type)
// =============================================================================

// RequirePermission checks if the user has a specific permission for a server
// This is the new PBAC middleware that uses the permissions package
func (s *Server) RequirePermission(perm permissions.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := s.getUserFromSession(c)
		if user == nil {
			responses.Unauthorized(c, "Unauthorized", nil)
			c.Abort()
			return
		}

		// Super admins have all permissions
		if user.SuperAdmin {
			c.Next()
			return
		}

		serverIdString := c.Param("serverId")
		if serverIdString == "" {
			responses.BadRequest(c, "Server ID is required", nil)
			c.Abort()
			return
		}

		serverId, err := uuid.Parse(serverIdString)
		if err != nil {
			responses.BadRequest(c, "Invalid server ID", nil)
			c.Abort()
			return
		}

		// Use new permission service
		hasPermission, err := s.Dependencies.PermissionService.HasPermission(c.Request.Context(), user.Id, serverId, perm)
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to check permissions: %w", err), nil)
			c.Abort()
			return
		}

		if !hasPermission {
			responses.Forbidden(c, "You don't have the required permission", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAnyPermission checks if the user has any of the specified permissions
func (s *Server) RequireAnyPermission(perms ...permissions.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := s.getUserFromSession(c)
		if user == nil {
			responses.Unauthorized(c, "Unauthorized", nil)
			c.Abort()
			return
		}

		if user.SuperAdmin {
			c.Next()
			return
		}

		serverIdString := c.Param("serverId")
		if serverIdString == "" {
			responses.BadRequest(c, "Server ID is required", nil)
			c.Abort()
			return
		}

		serverId, err := uuid.Parse(serverIdString)
		if err != nil {
			responses.BadRequest(c, "Invalid server ID", nil)
			c.Abort()
			return
		}

		hasPermission, err := s.Dependencies.PermissionService.HasAnyPermission(c.Request.Context(), user.Id, serverId, perms...)
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to check permissions: %w", err), nil)
			c.Abort()
			return
		}

		if !hasPermission {
			responses.Forbidden(c, "You don't have any of the required permissions", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAllPermissions checks if the user has all of the specified permissions
func (s *Server) RequireAllPermissions(perms ...permissions.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := s.getUserFromSession(c)
		if user == nil {
			responses.Unauthorized(c, "Unauthorized", nil)
			c.Abort()
			return
		}

		if user.SuperAdmin {
			c.Next()
			return
		}

		serverIdString := c.Param("serverId")
		if serverIdString == "" {
			responses.BadRequest(c, "Server ID is required", nil)
			c.Abort()
			return
		}

		serverId, err := uuid.Parse(serverIdString)
		if err != nil {
			responses.BadRequest(c, "Invalid server ID", nil)
			c.Abort()
			return
		}

		hasPermission, err := s.Dependencies.PermissionService.HasAllPermissions(c.Request.Context(), user.Id, serverId, perms...)
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to check permissions: %w", err), nil)
			c.Abort()
			return
		}

		if !hasPermission {
			responses.Forbidden(c, "You don't have all of the required permissions", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}
