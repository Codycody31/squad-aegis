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
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
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
			sa.eos_id,
			sa.server_role_id, 
			sa.expires_at,
			sa.notes,
			sa.created_at
		FROM server_admins sa
		WHERE sa.server_id = $1
	`, serverId)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer rows.Close()

	admins := []models.ServerAdmin{}

	for rows.Next() {
		var admin models.ServerAdmin

		err := rows.Scan(&admin.Id, &admin.ServerId, &admin.UserId, &admin.SteamId, &admin.EOSId, &admin.ServerRoleId, &admin.ExpiresAt, &admin.Notes, &admin.CreatedAt)
		if err != nil {
			responses.InternalServerError(c, err, nil)
			return
		}

		admins = append(admins, admin)
	}
	if err := rows.Err(); err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	// Prepare response with additional status information
	adminResponses := []gin.H{}
	for _, admin := range admins {
		adminResponse := gin.H{
			"id":             admin.Id,
			"server_id":      admin.ServerId,
			"user_id":        admin.UserId,
			"steam_id":       admin.SteamId,
			"eos_id":         admin.EOSId,
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

	hasUserID := request.UserID != nil && strings.TrimSpace(*request.UserID) != ""
	hasSteamID := request.SteamID != nil && strings.TrimSpace(*request.SteamID) != ""
	hasEOSID := request.EOSID != nil && strings.TrimSpace(*request.EOSID) != ""

	if !hasUserID && !hasSteamID && !hasEOSID {
		responses.BadRequest(c, "Either user_id, steam_id, or eos_id is required", &gin.H{"error": "Either user_id, steam_id, or eos_id is required"})
		return
	}

	if hasUserID && (hasSteamID || hasEOSID) {
		responses.BadRequest(c, "Cannot specify both user_id and a direct player ID", &gin.H{"error": "Cannot specify both user_id and a direct player ID"})
		return
	}

	if request.ServerRoleID == "" {
		responses.BadRequest(c, "Server role ID is required", &gin.H{"error": "Server role ID is required"})
		return
	}

	var targetUserID uuid.UUID
	var targetUser *models.User
	var identifiers utils.PlayerIdentifiers
	var steamIDArg interface{}
	var eosIDArg interface{}
	var userIDArg interface{}

	if hasUserID {
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
		identifiers = utils.NormalizePlayerIdentifiers("", fmt.Sprintf("%d", targetUser.SteamId), "")
		steamIDArg, eosIDArg, err = identifiers.DatabaseArgs()
		if err != nil {
			responses.BadRequest(c, "Failed to normalize user identifiers", &gin.H{"error": err.Error()})
			return
		}
	} else {
		requestSteamID := ""
		if request.SteamID != nil {
			requestSteamID = strings.TrimSpace(*request.SteamID)
		}
		requestEOSID := ""
		if request.EOSID != nil {
			requestEOSID = strings.TrimSpace(*request.EOSID)
		}
		inputIdentifiers := utils.NormalizePlayerIdentifiers(
			"",
			requestSteamID,
			requestEOSID,
		)

		if hasSteamID && inputIdentifiers.SteamID == "" {
			responses.BadRequest(c, "Invalid steam_id", &gin.H{"error": "steam_id must be a valid Steam ID"})
			return
		}
		if hasEOSID && inputIdentifiers.EOSID == "" {
			responses.BadRequest(c, "Invalid eos_id", &gin.H{"error": "eos_id must be a valid EOS ID"})
			return
		}

		identifiers = s.resolveCanonicalPlayerIdentifiers(c, inputIdentifiers.PlayerID, inputIdentifiers.SteamID, inputIdentifiers.EOSID)
		if identifiers.PlayerID == "" {
			responses.BadRequest(c, "Either steam_id or eos_id is required", &gin.H{"error": "Either steam_id or eos_id is required"})
			return
		}
		if hasSteamID && hasEOSID && (identifiers.SteamID == "" || identifiers.EOSID == "") {
			responses.BadRequest(c, "steam_id and eos_id must resolve to the same player", &gin.H{"error": "steam_id and eos_id must resolve to the same player"})
			return
		}

		steamIDArg, eosIDArg, err = identifiers.DatabaseArgs()
		if err != nil {
			responses.BadRequest(c, "Failed to normalize player identifiers", &gin.H{"error": err.Error()})
			return
		}

		targetUserID = uuid.Nil
		if steamIDArg != nil {
			err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
				SELECT id FROM users WHERE steam_id = $1
			`, steamIDArg).Scan(&targetUserID)
			if err == nil {
				targetUser, err = core.GetUserById(c.Request.Context(), s.Dependencies.DB, targetUserID, &user.Id)
				if err != nil {
					responses.BadRequest(c, "Failed to get user information", &gin.H{"error": err.Error()})
					return
				}
			} else if err != sql.ErrNoRows {
				responses.BadRequest(c, "Failed to check existing user", &gin.H{"error": err.Error()})
				return
			}
		}
	}

	if targetUserID != uuid.Nil {
		userIDArg = targetUserID
	}

	var adminID uuid.UUID
	var existingUserID sql.NullString

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT id, user_id
		FROM server_admins
		WHERE server_id = $1
		  AND server_role_id = $2
		  AND (
			($3::uuid IS NOT NULL AND user_id = $3::uuid)
			OR ($4::bigint IS NOT NULL AND steam_id = $4::bigint)
			OR ($5::text IS NOT NULL AND eos_id = $5::text)
		  )
		LIMIT 1
	`, serverId, request.ServerRoleID, userIDArg, steamIDArg, eosIDArg).Scan(&adminID, &existingUserID)

	if err == nil {
		if targetUserID != uuid.Nil && existingUserID.Valid && existingUserID.String != targetUserID.String() {
			responses.BadRequest(c, "Admin role already belongs to a different user", &gin.H{"error": "Admin role already belongs to a different user"})
			return
		}

		_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
			UPDATE server_admins
			SET user_id = COALESCE($1::uuid, user_id),
			    steam_id = COALESCE($2::bigint, steam_id),
			    eos_id = COALESCE(NULLIF($3::text, ''), eos_id),
			    expires_at = $4,
			    notes = $5
			WHERE id = $6
		`, userIDArg, steamIDArg, identifiers.EOSID, request.ExpiresAt, request.Notes, adminID)
		if err != nil {
			responses.InternalServerError(c, err, nil)
			return
		}
	} else if err == sql.ErrNoRows {
		adminID = uuid.New()
		_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
			INSERT INTO server_admins (id, server_id, user_id, steam_id, eos_id, server_role_id, expires_at, notes, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, adminID, serverId, userIDArg, steamIDArg, eosIDArg, request.ServerRoleID, request.ExpiresAt, request.Notes, time.Now())
		if err != nil {
			responses.InternalServerError(c, err, nil)
			return
		}
	} else {
		responses.InternalServerError(c, err, nil)
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
		responses.InternalServerError(c, err, nil)
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
	}
	if identifiers.SteamID != "" {
		auditData["steamId"] = identifiers.SteamID
		if targetUser == nil {
			auditData["username"] = fmt.Sprintf("Steam ID: %s", identifiers.SteamID)
		}
	}
	if identifiers.EOSID != "" {
		auditData["eosId"] = identifiers.EOSID
		if targetUser == nil && identifiers.SteamID == "" {
			auditData["username"] = fmt.Sprintf("EOS ID: %s", identifiers.EOSID)
		}
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:admin:create", auditData)

	responses.Success(c, "Admin created successfully", &gin.H{
		"adminId": adminID.String(),
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
	var existingSteamId sql.NullInt64
	var existingEOSId sql.NullString
	var existingRoleId uuid.UUID
	var existingNotes sql.NullString

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT user_id, steam_id, eos_id, server_role_id, notes
		FROM server_admins
		WHERE id = $1 AND server_id = $2
	`, adminId, serverId).Scan(&existingUserId, &existingSteamId, &existingEOSId, &existingRoleId, &existingNotes)

	if err != nil {
		if err == sql.ErrNoRows {
			responses.NotFound(c, "Admin not found", nil)
		} else {
			responses.InternalServerError(c, err, nil)
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
		responses.InternalServerError(c, err, nil)
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
		auditData["steamId"] = fmt.Sprintf("%d", existingSteamId.Int64)
		auditData["username"] = fmt.Sprintf("Steam ID: %d", existingSteamId.Int64)
	} else if existingEOSId.Valid {
		auditData["eosId"] = existingEOSId.String
		auditData["username"] = "EOS ID: " + existingEOSId.String
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
	var userId sql.NullString
	var steamID sql.NullInt64
	var eosID sql.NullString
	var roleId uuid.UUID
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT user_id, steam_id, eos_id, server_role_id FROM server_admins
		WHERE id = $1 AND server_id = $2
	`, adminId, serverId).Scan(&userId, &steamID, &eosID, &roleId)

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
		responses.InternalServerError(c, err, nil)
		return
	}

	// Create audit log with the information we have
	auditData := map[string]interface{}{
		"username": username,
		"roleId":   roleId.String(),
		"roleName": roleName,
	}
	if userId.Valid {
		auditData["userId"] = userId.String
	}
	if steamID.Valid {
		auditData["steamId"] = fmt.Sprintf("%d", steamID.Int64)
		auditData["username"] = fmt.Sprintf("Steam ID: %d", steamID.Int64)
	}
	if eosID.Valid {
		auditData["eosId"] = eosID.String
		auditData["username"] = "EOS ID: " + eosID.String
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:admin:remove", auditData)

	responses.Success(c, "Admin deleted successfully", nil)
}

// ServerAdminsCfg handles generating the admin config file
// This exports roles and admins in Squad's admin.cfg format
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
		responses.InternalServerError(c, err, nil)
		return
	}

	// Get all active role members for config generation (both admin and non-admin roles like whitelist, seeder, VIP)
	roleMembers, err := core.GetAllActiveServerRoleMembers(c.Request.Context(), s.Dependencies.DB, serverId)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	var configBuilder strings.Builder

	// Write role groups with their RCON permissions from new PBAC schema
	for _, role := range roles {
		// Get RCON permissions for this role from the new schema
		squadPerms, err := s.Dependencies.PermissionService.GetRCONPermissionsForExport(c.Request.Context(), role.Id)
		if err != nil {
			// Fall back to legacy permissions if new schema not available
			configBuilder.WriteString(fmt.Sprintf("Group=%s:%s\n", role.Name, strings.Join(role.Permissions, ",")))
			continue
		}

		if len(squadPerms) > 0 {
			configBuilder.WriteString(fmt.Sprintf("Group=%s:%s\n", role.Name, strings.Join(squadPerms, ",")))
		} else if len(role.Permissions) > 0 {
			// Fall back to legacy permissions
			configBuilder.WriteString(fmt.Sprintf("Group=%s:%s\n", role.Name, strings.Join(role.Permissions, ",")))
		}
	}

	// Write role member entries (admins, whitelist, seeders, etc.)
	for _, member := range roleMembers {
		roleName := ""

		for _, role := range roles {
			if role.Id == member.ServerRoleId {
				roleName = role.Name
				break
			}
		}

		if member.UserId != nil {
			user, err := core.GetUserById(c.Request.Context(), s.Dependencies.DB, *member.UserId, member.UserId)
			if err != nil {
				responses.BadRequest(c, "Failed to get user", &gin.H{"error": err.Error()})
				return
			}

			sid64 := steamid.New(user.SteamId)
			if !sid64.Valid() {
				continue
			}

			configBuilder.WriteString(fmt.Sprintf("Admin=%d:%s // %s\n", user.SteamId, roleName, user.Username))
		} else if member.SteamId != nil {
			configBuilder.WriteString(fmt.Sprintf("Admin=%d:%s // Unknown\n", *member.SteamId, roleName))
		} else if member.EOSId != nil {
			configBuilder.WriteString(fmt.Sprintf("Admin=%s:%s // Unknown\n", *member.EOSId, roleName))
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
