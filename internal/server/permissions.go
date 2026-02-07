package server

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// PermissionsList returns all available permissions grouped by category
func (s *Server) PermissionsList(c *gin.Context) {
	perms, err := s.Dependencies.PermissionRepo.GetPermissionsGrouped(c.Request.Context())
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "Permissions retrieved", &gin.H{
		"permissions": perms,
	})
}

// RoleTemplatesList returns all available role templates with their permissions
func (s *Server) RoleTemplatesList(c *gin.Context) {
	templates, err := s.Dependencies.PermissionRepo.GetRoleTemplatesWithPermissions(c.Request.Context())
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "Role templates retrieved", &gin.H{
		"templates": templates,
	})
}

// ServerRolePermissionsGet retrieves permissions for a specific server role
func (s *Server) ServerRolePermissionsGet(c *gin.Context) {
	roleIdStr := c.Param("roleId")
	roleId, err := uuid.Parse(roleIdStr)
	if err != nil {
		responses.BadRequest(c, "Invalid role ID", nil)
		return
	}

	perms, err := s.Dependencies.PermissionRepo.GetServerRolePermissions(c.Request.Context(), roleId)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "Role permissions retrieved", &gin.H{
		"permissions": perms,
	})
}

// ServerRolePermissionsUpdate updates permissions for a specific server role
type UpdateRolePermissionsRequest struct {
	Permissions []string `json:"permissions" binding:"required"`
}

func (s *Server) ServerRolePermissionsUpdate(c *gin.Context) {
	roleIdStr := c.Param("roleId")
	roleId, err := uuid.Parse(roleIdStr)
	if err != nil {
		responses.BadRequest(c, "Invalid role ID", nil)
		return
	}

	var req UpdateRolePermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request payload", nil)
		return
	}

	// Update permissions
	err = s.Dependencies.PermissionRepo.SetServerRolePermissions(c.Request.Context(), roleId, req.Permissions)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	// Invalidate cache for all users on this server
	serverIdStr := c.Param("serverId")
	if serverId, err := uuid.Parse(serverIdStr); err == nil {
		s.Dependencies.PermissionService.InvalidateServerCache(serverId)
	}

	responses.SimpleSuccess(c, "Role permissions updated")
}

// ServerRoleCreateFromTemplate creates a new server role from a template
type CreateRoleFromTemplateRequest struct {
	TemplateId uuid.UUID `json:"template_id" binding:"required"`
	Name       string    `json:"name" binding:"required"`
}

func (s *Server) ServerRoleCreateFromTemplate(c *gin.Context) {
	serverIdStr := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdStr)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", nil)
		return
	}

	var req CreateRoleFromTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request payload", nil)
		return
	}

	// Get the template to determine is_admin status
	template, err := s.Dependencies.PermissionRepo.GetRoleTemplateById(c.Request.Context(), req.TemplateId)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	if template == nil {
		responses.NotFound(c, "Role template not found", nil)
		return
	}

	// Create the role in a transaction
	tx, err := s.Dependencies.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer tx.Rollback()

	// Insert the new role
	roleId := uuid.New()
	_, err = tx.ExecContext(c.Request.Context(), `
		INSERT INTO server_roles (id, server_id, name, permissions, is_admin, created_at)
		VALUES ($1, $2, $3, '', $4, NOW())
	`, roleId, serverId, req.Name, template.IsAdmin)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	// Copy permissions from template
	_, err = tx.ExecContext(c.Request.Context(), `
		INSERT INTO server_role_permissions (server_role_id, permission_id)
		SELECT $1, permission_id FROM role_template_permissions WHERE role_template_id = $2
	`, roleId, req.TemplateId)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	if err := tx.Commit(); err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "Role created from template", &gin.H{
		"role_id": roleId,
	})
}
