package server

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// ServerRolesList handles listing all roles for a server
func (s *Server) ServerRolesList(c *gin.Context) {
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

	// Query the database for roles
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT id, server_id, name, permissions, is_admin, created_at
		FROM server_roles
		WHERE server_id = $1
		ORDER BY name ASC
	`, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to query roles", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	roles := []models.ServerRole{}

	for rows.Next() {
		var role models.ServerRole
		var permissionsStr string

		err := rows.Scan(&role.Id, &role.ServerId, &role.Name, &permissionsStr, &role.IsAdmin, &role.CreatedAt)
		if err != nil {
			responses.BadRequest(c, "Failed to scan role", &gin.H{"error": err.Error()})
			return
		}

		// Parse permissions from comma-separated string
		role.Permissions = strings.Split(permissionsStr, ",")
		roles = append(roles, role)
	}

	responses.Success(c, "Roles fetched successfully", &gin.H{
		"roles": roles,
	})
}

// ServerRolesAdd handles adding a new role
func (s *Server) ServerRolesAdd(c *gin.Context) {
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

	var request models.ServerRoleCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if request.Name == "" {
		responses.BadRequest(c, "Role name is required", &gin.H{"error": "Role name is required"})
		return
	}

	// Ensure name has no spaces, only allows alphanumeric and underscores
	matched, err := regexp.MatchString("^[a-zA-Z0-9_]+$", request.Name)
	if err != nil {
		responses.BadRequest(c, "Failed to validate role name", &gin.H{"error": err.Error()})
		return
	}

	if !matched {
		responses.BadRequest(c, "Role name can only contain alphanumeric characters and underscores", &gin.H{"error": "Role name can only contain alphanumeric characters and underscores"})
		return
	}

	if len(request.Permissions) == 0 {
		responses.BadRequest(c, "At least one permission is required", &gin.H{"error": "At least one permission is required"})
		return
	}

	// Convert permissions array to comma-separated string
	permissionsStr := strings.Join(request.Permissions, ",")

	// Default is_admin to true if not provided
	isAdmin := true
	if request.IsAdmin != nil {
		isAdmin = *request.IsAdmin
	}

	// Insert the role into the database
	var roleID string
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		INSERT INTO server_roles (id, server_id, name, permissions, is_admin, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, uuid.New(), serverId, request.Name, permissionsStr, isAdmin, time.Now()).Scan(&roleID)

	if err != nil {
		responses.BadRequest(c, "Failed to create role", &gin.H{"error": err.Error()})
		return
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:role:create", map[string]interface{}{
		"name":        request.Name,
		"permissions": request.Permissions,
		"is_admin":    isAdmin,
		"roleId":      roleID,
	})

	responses.Success(c, "Role created successfully", &gin.H{
		"roleId": roleID,
	})
}

// ServerRolesRemove handles removing a role
func (s *Server) ServerRolesRemove(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	roleIdString := c.Param("roleId")
	roleId, err := uuid.Parse(roleIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid role ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}
	_ = server // Ensure server is used

	// Check if role is in use by any admins
	var count int
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT COUNT(*) FROM server_admins
		WHERE server_role_id = $1
	`, roleId).Scan(&count)

	if err != nil {
		responses.BadRequest(c, "Failed to check if role is in use", &gin.H{"error": err.Error()})
		return
	}

	if count > 0 {
		responses.BadRequest(c, "Role is in use by admins and cannot be removed", &gin.H{"error": "Role is in use by admins and cannot be removed"})
		return
	}

	// Get the role name
	var name string
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT name FROM server_roles WHERE id = $1
	`, roleId).Scan(&name)

	if err != nil {
		responses.BadRequest(c, "Failed to get role name", &gin.H{"error": err.Error()})
		return
	}

	// Delete the role
	_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
		DELETE FROM server_roles
		WHERE id = $1 AND server_id = $2
	`, roleId, serverId)

	if err != nil {
		responses.BadRequest(c, "Failed to delete role", &gin.H{"error": err.Error()})
		return
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:role:delete", map[string]interface{}{
		"name":   name,
		"roleId": roleId.String(),
	})

	responses.Success(c, "Role deleted successfully", nil)
}

// ServerRolesUpdate handles updating a role
func (s *Server) ServerRolesUpdate(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	roleIdString := c.Param("roleId")
	roleId, err := uuid.Parse(roleIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid role ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}
	_ = server // Ensure server is used

	var request models.ServerRoleUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Get existing role to check if it exists and for audit log
	var existingName string
	var existingPermissionsStr string
	var existingIsAdmin bool
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT name, permissions, is_admin FROM server_roles
		WHERE id = $1 AND server_id = $2
	`, roleId, serverId).Scan(&existingName, &existingPermissionsStr, &existingIsAdmin)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			responses.NotFound(c, "Role not found", nil)
		} else {
			responses.BadRequest(c, "Failed to get role", &gin.H{"error": err.Error()})
		}
		return
	}

	// Build update query dynamically based on what fields are provided
	updateFields := []string{}
	args := []interface{}{}
	argIndex := 1

	if request.Name != nil {
		// Validate name if provided
		matched, err := regexp.MatchString("^[a-zA-Z0-9_]+$", *request.Name)
		if err != nil {
			responses.BadRequest(c, "Failed to validate role name", &gin.H{"error": err.Error()})
			return
		}
		if !matched {
			responses.BadRequest(c, "Role name can only contain alphanumeric characters and underscores", &gin.H{"error": "Role name can only contain alphanumeric characters and underscores"})
			return
		}
		updateFields = append(updateFields, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, *request.Name)
		argIndex++
	}

	if request.Permissions != nil {
		if len(request.Permissions) == 0 {
			responses.BadRequest(c, "At least one permission is required", &gin.H{"error": "At least one permission is required"})
			return
		}
		permissionsStr := strings.Join(request.Permissions, ",")
		updateFields = append(updateFields, fmt.Sprintf("permissions = $%d", argIndex))
		args = append(args, permissionsStr)
		argIndex++
	}

	if request.IsAdmin != nil {
		updateFields = append(updateFields, fmt.Sprintf("is_admin = $%d", argIndex))
		args = append(args, *request.IsAdmin)
		argIndex++
	}

	if len(updateFields) == 0 {
		responses.BadRequest(c, "No fields to update", &gin.H{"error": "No fields to update"})
		return
	}

	// Add roleId and serverId to args for WHERE clause
	args = append(args, roleId, serverId)

	// Build and execute update query
	query := fmt.Sprintf(`
		UPDATE server_roles
		SET %s
		WHERE id = $%d AND server_id = $%d
	`, strings.Join(updateFields, ", "), argIndex, argIndex+1)

	_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), query, args...)
	if err != nil {
		responses.BadRequest(c, "Failed to update role", &gin.H{"error": err.Error()})
		return
	}

	// Prepare audit log data
	auditData := map[string]interface{}{
		"roleId": roleId.String(),
		"name":   existingName,
	}

	if request.Name != nil && *request.Name != existingName {
		auditData["oldName"] = existingName
		auditData["newName"] = *request.Name
	}

	if request.Permissions != nil {
		auditData["oldPermissions"] = strings.Split(existingPermissionsStr, ",")
		auditData["newPermissions"] = request.Permissions
	}

	if request.IsAdmin != nil && *request.IsAdmin != existingIsAdmin {
		auditData["oldIsAdmin"] = existingIsAdmin
		auditData["newIsAdmin"] = *request.IsAdmin
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:role:update", auditData)

	responses.Success(c, "Role updated successfully", nil)
}
