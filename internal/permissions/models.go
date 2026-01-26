package permissions

import (
	"time"

	"github.com/google/uuid"
)

// PermissionDefinition represents a permission in the database.
type PermissionDefinition struct {
	Id              uuid.UUID `json:"id"`
	Code            string    `json:"code"`
	Category        string    `json:"category"`
	Name            string    `json:"name"`
	Description     *string   `json:"description,omitempty"`
	SquadPermission *string   `json:"squad_permission,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// RoleTemplate represents a predefined role template.
type RoleTemplate struct {
	Id          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	IsSystem    bool      `json:"is_system"`
	IsAdmin     bool      `json:"is_admin"`
	CreatedAt   time.Time `json:"created_at"`
}

// RoleTemplateWithPermissions includes the permissions for a role template.
type RoleTemplateWithPermissions struct {
	RoleTemplate
	Permissions []string `json:"permissions"`
}

// PermissionGroup represents a grouped set of permissions for UI display.
type PermissionGroup struct {
	Category    string                 `json:"category"`
	Name        string                 `json:"name"`
	Permissions []PermissionDefinition `json:"permissions"`
}
