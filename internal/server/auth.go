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

	// Get user's server permissions
	serverPermissions, err := core.GetUserServerPermissions(c.Copy(), s.Dependencies.DB, session.UserId)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "User authenticated", &gin.H{
		"user":              user,
		"serverPermissions": serverPermissions,
	})
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

		// Get the user's permissions for this server
		psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
		sql, args, err := psql.Select("sr.permissions").
			From("server_admins sa").
			Join("server_roles sr ON sa.server_role_id = sr.id").
			Where(squirrel.Eq{"sa.server_id": serverId, "sa.user_id": user.Id}).
			ToSql()
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to create SQL query: %w", err), nil)
			c.Abort()
			return
		}

		var permissionsStr string
		err = s.Dependencies.DB.QueryRowContext(c.Copy(), sql, args...).Scan(&permissionsStr)
		if err != nil {
			if strings.Contains(err.Error(), "no rows") {
				responses.Forbidden(c, "You don't have permission to access this server", nil)
				c.Abort()
				return
			}
			responses.InternalServerError(c, fmt.Errorf("failed to get permissions: %w", err), nil)
			c.Abort()
			return
		}

		// Parse permissions from comma-separated string
		permissions := strings.Split(permissionsStr, ",")

		// Check if the user has the required permission
		hasPermission := false
		for _, p := range permissions {
			if p == permission || p == "*" {
				hasPermission = true
				break
			}
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
func (s *Server) AuthHasAnyServerPermission(permissions ...string) gin.HandlerFunc {
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

		// Get the user's permissions for this server
		psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
		sql, args, err := psql.Select("sr.permissions").
			From("server_admins sa").
			Join("server_roles sr ON sa.server_role_id = sr.id").
			Where(squirrel.Eq{"sa.server_id": serverId, "sa.user_id": user.Id}).
			ToSql()
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to create SQL query: %w", err), nil)
			c.Abort()
			return
		}

		var permissionsStr string
		err = s.Dependencies.DB.QueryRowContext(c.Copy(), sql, args...).Scan(&permissionsStr)
		if err != nil {
			if strings.Contains(err.Error(), "no rows") {
				responses.Forbidden(c, "You don't have permission to access this server", nil)
				c.Abort()
				return
			}
			responses.InternalServerError(c, fmt.Errorf("failed to get permissions: %w", err), nil)
			c.Abort()
			return
		}

		// Parse permissions from comma-separated string
		userPermissions := strings.Split(permissionsStr, ",")

		// Check if the user has any of the required permissions
		hasPermission := false
		for _, userPerm := range userPermissions {
			if userPerm == "*" {
				hasPermission = true
				break
			}
			for _, requiredPerm := range permissions {
				if userPerm == requiredPerm {
					hasPermission = true
					break
				}
			}
			if hasPermission {
				break
			}
		}

		if !hasPermission {
			responses.Forbidden(c, "You don't have any of the required permissions", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}
