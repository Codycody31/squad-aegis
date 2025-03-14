package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/core"
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
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}
	_ = server // Ensure server is used

	// Query the database for admins
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT sa.id, sa.server_id, sa.user_id, u.username, sa.server_role_id, sr.name as role_name, sa.created_at
		FROM server_admins sa
		JOIN users u ON sa.user_id = u.id
		JOIN server_roles sr ON sa.server_role_id = sr.id
		WHERE sa.server_id = $1
		ORDER BY u.username ASC
	`, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to query admins", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	admins := []ServerAdmin{}

	for rows.Next() {
		var admin ServerAdmin
		err := rows.Scan(&admin.ID, &admin.ServerID, &admin.UserID, &admin.Username, &admin.ServerRoleID, &admin.RoleName, &admin.CreatedAt)
		if err != nil {
			responses.BadRequest(c, "Failed to scan admin", &gin.H{"error": err.Error()})
			return
		}

		admins = append(admins, admin)
	}

	responses.Success(c, "Admins fetched successfully", &gin.H{
		"admins": admins,
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

	var request ServerAdminCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	if request.UserID == "" {
		responses.BadRequest(c, "User ID is required", &gin.H{"error": "User ID is required"})
		return
	}

	if request.ServerRoleID == "" {
		responses.BadRequest(c, "Server role ID is required", &gin.H{"error": "Server role ID is required"})
		return
	}

	// Check if user already exists as admin for this server
	var count int
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT COUNT(*) FROM server_admins
		WHERE server_id = $1 AND user_id = $2
	`, serverId, request.UserID).Scan(&count)

	if err != nil {
		responses.BadRequest(c, "Failed to check if user is already an admin", &gin.H{"error": err.Error()})
		return
	}

	if count > 0 {
		responses.BadRequest(c, "User is already an admin for this server", &gin.H{"error": "User is already an admin for this server"})
		return
	}

	// Insert the admin into the database
	var adminID string
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		INSERT INTO server_admins (server_id, user_id, server_role_id)
		VALUES ($1, $2, $3)
		RETURNING id
	`, serverId, request.UserID, request.ServerRoleID).Scan(&adminID)

	if err != nil {
		responses.BadRequest(c, "Failed to create admin", &gin.H{"error": err.Error()})
		return
	}

	// Get user information for audit log
	userUUID, err := uuid.Parse(request.UserID)
	if err != nil {
		responses.BadRequest(c, "Invalid user ID", &gin.H{"error": err.Error()})
		return
	}

	targetUser, err := core.GetUserById(c.Request.Context(), s.Dependencies.DB, userUUID, &user.Id)
	if err != nil {
		responses.BadRequest(c, "Failed to get user information", &gin.H{"error": err.Error()})
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

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:admin:create", map[string]interface{}{
		"userId":   request.UserID,
		"username": targetUser.Username,
		"roleId":   request.ServerRoleID,
		"roleName": roleName,
	})

	responses.Success(c, "Admin created successfully", &gin.H{
		"adminId": adminID,
	})
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

	// Get admins
	admins, err := core.GetServerAdmins(c.Request.Context(), s.Dependencies.DB, serverId)
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

		user, err := core.GetUserById(c.Request.Context(), s.Dependencies.DB, admin.UserId, &admin.UserId)
		if err != nil {
			responses.BadRequest(c, "Failed to get user", &gin.H{"error": err.Error()})
			return
		}

		if user.SteamId == 0 {
			continue
		}

		configBuilder.WriteString(fmt.Sprintf("Admin=%d:%s\n", user.SteamId, roleName))
	}

	// Set the content type and send the response
	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, configBuilder.String())
}
