package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/leighmacdonald/steamid/v3/steamid"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// ServerAdminsList handles listing all admins for a server
func (s *Server) ServerAdminsList(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Query the database for admins (handle both user_id and steam_id cases)
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT 
			sa.id, 
			sa.server_id, 
			sa.user_id, 
			sa.steam_id,
			sa.server_role_id, 
			sa.expires_at,
			sa.notes,
			sa.created_at
		FROM server_admins sa
		WHERE sa.server_id = $1
	`, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to query admins", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	admins := []models.ServerAdmin{}

	for rows.Next() {
		var admin models.ServerAdmin

		err := rows.Scan(&admin.Id, &admin.ServerId, &admin.UserId, &admin.SteamId, &admin.ServerRoleId, &admin.ExpiresAt, &admin.Notes, &admin.CreatedAt)
		if err != nil {
			responses.BadRequest(c, "Failed to scan admin", &gin.H{"error": err.Error()})
			return
		}

		admins = append(admins, admin)
	}

	// Prepare response with additional status information
	adminResponses := []gin.H{}
	for _, admin := range admins {
		adminResponse := gin.H{
			"id":             admin.Id,
			"server_id":      admin.ServerId,
			"user_id":        admin.UserId,
			"steam_id":       admin.SteamId,
			"server_role_id": admin.ServerRoleId,
			"expires_at":     admin.ExpiresAt,
			"notes":          admin.Notes,
			"created_at":     admin.CreatedAt,
			"is_active":      admin.IsActive(),
			"is_expired":     admin.IsExpired(),
		}
		adminResponses = append(adminResponses, adminResponse)
	}

	responses.Success(c, "Admins fetched successfully", &gin.H{
		"admins": adminResponses,
	})
}

// ServerAdminsAdd handles adding a new admin
func (s *Server) ServerAdminsAdd(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}
	_ = server // Ensure server is used

	var request models.ServerAdminCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Validate that either UserID or SteamID is provided, but not both
	if (request.UserID == nil || *request.UserID == "") && (request.SteamID == nil || *request.SteamID == "") {
		responses.BadRequest(c, "Either User ID or Steam ID is required", &gin.H{"error": "Either User ID or Steam ID is required"})
		return
	}

	if (request.UserID != nil && *request.UserID != "") && (request.SteamID != nil && *request.SteamID != "") {
		responses.BadRequest(c, "Cannot specify both User ID and Steam ID", &gin.H{"error": "Cannot specify both User ID and Steam ID"})
		return
	}

	if request.ServerRoleID == "" {
		responses.BadRequest(c, "Server role ID is required", &gin.H{"error": "Server role ID is required"})
		return
	}

	var targetUserID uuid.UUID
	var targetUser *models.User
	var steamID *string

	// Handle existing user case
	if request.UserID != nil && *request.UserID != "" {
		userUUID, err := uuid.Parse(*request.UserID)
		if err != nil {
			responses.BadRequest(c, "Invalid user ID", &gin.H{"error": err.Error()})
			return
		}

		targetUser, err = core.GetUserById(c.Request.Context(), s.Dependencies.DB, userUUID, &user.Id)
		if err != nil {
			responses.BadRequest(c, "Failed to get user information", &gin.H{"error": err.Error()})
			return
		}
		targetUserID = userUUID

		// Check if user already exists as admin for this server
		var count int
		err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
			SELECT COUNT(*) FROM server_admins
			WHERE server_id = $1 AND user_id = $2
		`, serverId, targetUserID).Scan(&count)

		if err != nil {
			responses.BadRequest(c, "Failed to check if user is already an admin", &gin.H{"error": err.Error()})
			return
		}

		if count > 0 {
			responses.BadRequest(c, "User is already an admin for this server", &gin.H{"error": "User is already an admin for this server"})
			return
		}
	} else {
		// Handle Steam ID case - check if user with this Steam ID already exists
		steamID = request.SteamID

		err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
			SELECT id FROM users WHERE steam_id = $1
		`, *steamID).Scan(&targetUserID)

		if err == sql.ErrNoRows {
			// User doesn't exist, we'll store the Steam ID directly in server_admins
			targetUserID = uuid.Nil
		} else if err != nil {
			responses.BadRequest(c, "Failed to check existing user", &gin.H{"error": err.Error()})
			return
		} else {
			// User exists, get their information
			targetUser, err = core.GetUserById(c.Request.Context(), s.Dependencies.DB, targetUserID, &user.Id)
			if err != nil {
				responses.BadRequest(c, "Failed to get user information", &gin.H{"error": err.Error()})
				return
			}

			// Check if user already exists as admin for this server
			var count int
			err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
				SELECT COUNT(*) FROM server_admins
				WHERE server_id = $1 AND user_id = $2
			`, serverId, targetUserID).Scan(&count)

			if err != nil {
				responses.BadRequest(c, "Failed to check if user is already an admin", &gin.H{"error": err.Error()})
				return
			}

			if count > 0 {
				responses.BadRequest(c, "User is already an admin for this server", &gin.H{"error": "User is already an admin for this server"})
				return
			}
		}

		// Also check if Steam ID is already used as admin for this server
		var count int
		err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
			SELECT COUNT(*) FROM server_admins
			WHERE server_id = $1 AND steam_id = $2
		`, serverId, *steamID).Scan(&count)

		if err != nil {
			responses.BadRequest(c, "Failed to check if Steam ID is already an admin", &gin.H{"error": err.Error()})
			return
		}

		if count > 0 {
			responses.BadRequest(c, "Steam ID is already an admin for this server", &gin.H{"error": "Steam ID is already an admin for this server"})
			return
		}
	}

	// Insert the admin into the database
	var adminID string
	var query string
	var args []interface{}

	if targetUserID != uuid.Nil {
		// User exists, use user_id
		query = `
			INSERT INTO server_admins (id, server_id, user_id, server_role_id, expires_at, notes, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id
		`
		args = []interface{}{uuid.New(), serverId, targetUserID, request.ServerRoleID, request.ExpiresAt, request.Notes, time.Now()}
	} else {
		// User doesn't exist, use steam_id
		query = `
			INSERT INTO server_admins (id, server_id, steam_id, server_role_id, expires_at, notes, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id
		`
		args = []interface{}{uuid.New(), serverId, *steamID, request.ServerRoleID, request.ExpiresAt, request.Notes, time.Now()}
	}

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), query, args...).Scan(&adminID)
	if err != nil {
		responses.BadRequest(c, "Failed to create admin", &gin.H{"error": err.Error()})
		return
	}

	// Get role information for audit log
	roleUUID, err := uuid.Parse(request.ServerRoleID)
	if err != nil {
		responses.BadRequest(c, "Invalid role ID", &gin.H{"error": err.Error()})
		return
	}

	// Find the role name from the server roles
	roles, err := core.GetServerRoles(c.Request.Context(), s.Dependencies.DB, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to get server roles", &gin.H{"error": err.Error()})
		return
	}

	roleName := ""
	for _, role := range roles {
		if role.Id == roleUUID {
			roleName = role.Name
			break
		}
	}

	// Create audit log
	auditData := map[string]interface{}{
		"adminId":  adminID,
		"roleId":   request.ServerRoleID,
		"roleName": roleName,
	}

	// Add expiration information to audit log
	if request.ExpiresAt != nil {
		auditData["expiresAt"] = request.ExpiresAt.Format(time.RFC3339)
	}

	// Add notes to audit log
	if request.Notes != nil && *request.Notes != "" {
		auditData["notes"] = *request.Notes
	}

	// Add user information to audit log
	if targetUser != nil {
		auditData["userId"] = targetUser.Id.String()
		auditData["username"] = targetUser.Username
	} else if steamID != nil {
		auditData["steamId"] = *steamID
		auditData["username"] = fmt.Sprintf("Steam ID: %s", *steamID)
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:admin:create", auditData)

	responses.Success(c, "Admin created successfully", &gin.H{
		"adminId": adminID,
	})
}

// ServerAdminsUpdate handles updating an admin's notes
func (s *Server) ServerAdminsUpdate(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	adminIdString := c.Param("adminId")
	adminId, err := uuid.Parse(adminIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid admin ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Parse request body
	var updateData struct {
		Notes string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		responses.BadRequest(c, "Invalid request data", &gin.H{"error": err.Error()})
		return
	}

	// Get existing admin details for audit log
	var existingUserId sql.NullString
	var existingSteamId sql.NullString
	var existingRoleId uuid.UUID
	var existingNotes sql.NullString

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT user_id, steam_id, server_role_id, notes
		FROM server_admins
		WHERE id = $1 AND server_id = $2
	`, adminId, serverId).Scan(&existingUserId, &existingSteamId, &existingRoleId, &existingNotes)

	if err != nil {
		if err == sql.ErrNoRows {
			responses.NotFound(c, "Admin not found", nil)
		} else {
			responses.BadRequest(c, "Failed to get admin", &gin.H{"error": err.Error()})
		}
		return
	}

	// Update the admin's notes
	var notes *string
	if updateData.Notes != "" {
		notes = &updateData.Notes
	}

	_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
		UPDATE server_admins 
		SET notes = $1
		WHERE id = $2 AND server_id = $3
	`, notes, adminId, serverId)

	if err != nil {
		responses.BadRequest(c, "Failed to update admin", &gin.H{"error": err.Error()})
		return
	}

	// Create audit log
	auditData := map[string]interface{}{
		"adminId": adminId.String(),
		"roleId":  existingRoleId.String(),
	}

	// Get username for audit log
	if existingUserId.Valid {
		auditData["userId"] = existingUserId.String
		// TODO: Get actual username from users table if needed
		auditData["username"] = "User ID: " + existingUserId.String
	} else if existingSteamId.Valid {
		auditData["steamId"] = existingSteamId.String
		auditData["username"] = "Steam ID: " + existingSteamId.String
	}

	// Add notes change information
	oldNotes := ""
	if existingNotes.Valid {
		oldNotes = existingNotes.String
	}
	auditData["oldNotes"] = oldNotes
	auditData["newNotes"] = updateData.Notes

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:admin:update", auditData)

	responses.Success(c, "Admin updated successfully", nil)
}

// ServerAdminsRemove handles removing an admin
func (s *Server) ServerAdminsRemove(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	adminIdString := c.Param("adminId")
	adminId, err := uuid.Parse(adminIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid admin ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}
	_ = server // Ensure server is used

	// Get admin details before deletion for audit log
	var userId uuid.UUID
	var roleId uuid.UUID
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT user_id, server_role_id FROM server_admins
		WHERE id = $1 AND server_id = $2
	`, adminId, serverId).Scan(&userId, &roleId)

	// Store admin details for audit log
	var username string = "Unknown"
	var roleName string = "Unknown"

	if err == nil {
		// Get role information
		roles, roleErr := core.GetServerRoles(c.Request.Context(), s.Dependencies.DB, serverId)
		if roleErr == nil && roles != nil {
			for _, role := range roles {
				if role.Id == roleId {
					roleName = role.Name
					break
				}
			}
		}
	}

	// Delete the admin
	_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
		DELETE FROM server_admins
		WHERE id = $1 AND server_id = $2
	`, adminId, serverId)

	if err != nil {
		responses.BadRequest(c, "Failed to delete admin", &gin.H{"error": err.Error()})
		return
	}

	// Create audit log with the information we have
	auditData := map[string]interface{}{
		"userId":   userId.String(),
		"username": username,
		"roleId":   roleId.String(),
		"roleName": roleName,
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:admin:remove", auditData)

	responses.Success(c, "Admin deleted successfully", nil)
}

// ServerAdminsCfg handles generating the admin config file
func (s *Server) ServerAdminsCfg(c *gin.Context) {
	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Get roles
	roles, err := core.GetServerRoles(c.Request.Context(), s.Dependencies.DB, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to get server roles", &gin.H{"error": err.Error()})
		return
	}

	// Get admins (only active ones for config generation)
	admins, err := core.GetActiveServerAdmins(c.Request.Context(), s.Dependencies.DB, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to get server admins", &gin.H{"error": err.Error()})
		return
	}

	var configBuilder strings.Builder

	for _, role := range roles {
		configBuilder.WriteString(fmt.Sprintf("Group=%s:%s\n", role.Name, strings.Join(role.Permissions, ",")))
	}

	for _, admin := range admins {
		roleName := ""

		for _, role := range roles {
			if role.Id == admin.ServerRoleId {
				roleName = role.Name
				break
			}
		}

		if admin.UserId != nil {
			user, err := core.GetUserById(c.Request.Context(), s.Dependencies.DB, *admin.UserId, admin.UserId)
			if err != nil {
				responses.BadRequest(c, "Failed to get user", &gin.H{"error": err.Error()})
				return
			}

			sid64 := steamid.New(user.SteamId)
			if !sid64.Valid() {
				continue
			}

			configBuilder.WriteString(fmt.Sprintf("Admin=%d:%s // %s\n", user.SteamId, roleName, user.Username))
		} else if admin.SteamId != nil {
			configBuilder.WriteString(fmt.Sprintf("Admin=%d:%s // Unknown\n", *admin.SteamId, roleName))
		}
	}

	// Set the content type and send the response
	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, configBuilder.String())
}

// ServerAdminsCleanupExpired handles manual cleanup of expired admin roles
func (s *Server) ServerAdminsCleanupExpired(c *gin.Context) {
	user := s.getUserFromSession(c)

	// Clean up expired admins
	deleted, err := core.CleanupExpiredAdmins(c.Request.Context(), s.Dependencies.DB)
	if err != nil {
		responses.BadRequest(c, "Failed to cleanup expired admins", &gin.H{"error": err.Error()})
		return
	}

	// Create audit log for the cleanup action
	auditData := map[string]interface{}{
		"deletedCount": deleted,
		"action":       "manual_cleanup",
	}

	s.CreateAuditLog(c.Request.Context(), nil, &user.Id, "admin:cleanup:expired", auditData)

	responses.Success(c, "Expired admins cleaned up successfully", &gin.H{
		"deletedCount": deleted,
	})
}
