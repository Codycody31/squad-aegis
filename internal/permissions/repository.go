package permissions

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

// Repository provides database operations for permissions.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new permission repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetAllPermissions retrieves all permission definitions from the database.
func (r *Repository) GetAllPermissions(ctx context.Context) ([]PermissionDefinition, error) {
	query := `
		SELECT id, code, category, name, description, squad_permission, created_at
		FROM permissions
		ORDER BY category, code
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []PermissionDefinition
	for rows.Next() {
		var p PermissionDefinition
		if err := rows.Scan(&p.Id, &p.Code, &p.Category, &p.Name, &p.Description, &p.SquadPermission, &p.CreatedAt); err != nil {
			return nil, err
		}
		permissions = append(permissions, p)
	}

	return permissions, rows.Err()
}

// GetPermissionsByCategory retrieves permissions filtered by category.
func (r *Repository) GetPermissionsByCategory(ctx context.Context, category string) ([]PermissionDefinition, error) {
	query := `
		SELECT id, code, category, name, description, squad_permission, created_at
		FROM permissions
		WHERE category = $1
		ORDER BY code
	`

	rows, err := r.db.QueryContext(ctx, query, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []PermissionDefinition
	for rows.Next() {
		var p PermissionDefinition
		if err := rows.Scan(&p.Id, &p.Code, &p.Category, &p.Name, &p.Description, &p.SquadPermission, &p.CreatedAt); err != nil {
			return nil, err
		}
		permissions = append(permissions, p)
	}

	return permissions, rows.Err()
}

// GetPermissionByCode retrieves a single permission by its code.
func (r *Repository) GetPermissionByCode(ctx context.Context, code string) (*PermissionDefinition, error) {
	query := `
		SELECT id, code, category, name, description, squad_permission, created_at
		FROM permissions
		WHERE code = $1
	`

	var p PermissionDefinition
	err := r.db.QueryRowContext(ctx, query, code).Scan(
		&p.Id, &p.Code, &p.Category, &p.Name, &p.Description, &p.SquadPermission, &p.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &p, nil
}

// GetPermissionsGrouped retrieves all permissions grouped by category.
func (r *Repository) GetPermissionsGrouped(ctx context.Context) ([]PermissionGroup, error) {
	perms, err := r.GetAllPermissions(ctx)
	if err != nil {
		return nil, err
	}

	categoryNames := map[string]string{
		"system": "System",
		"ui":     "User Interface",
		"api":    "API Access",
		"rcon":   "RCON / Squad Server",
	}

	groups := make(map[string]*PermissionGroup)
	order := []string{"ui", "api", "rcon", "system"}

	for _, cat := range order {
		groups[cat] = &PermissionGroup{
			Category:    cat,
			Name:        categoryNames[cat],
			Permissions: []PermissionDefinition{},
		}
	}

	for _, p := range perms {
		if group, ok := groups[p.Category]; ok {
			group.Permissions = append(group.Permissions, p)
		}
	}

	var result []PermissionGroup
	for _, cat := range order {
		if len(groups[cat].Permissions) > 0 {
			result = append(result, *groups[cat])
		}
	}

	return result, nil
}

// GetAllRoleTemplates retrieves all role templates.
func (r *Repository) GetAllRoleTemplates(ctx context.Context) ([]RoleTemplate, error) {
	query := `
		SELECT id, name, description, is_system, is_admin, created_at
		FROM role_templates
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []RoleTemplate
	for rows.Next() {
		var t RoleTemplate
		if err := rows.Scan(&t.Id, &t.Name, &t.Description, &t.IsSystem, &t.IsAdmin, &t.CreatedAt); err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}

	return templates, rows.Err()
}

// GetRoleTemplateById retrieves a role template by ID.
func (r *Repository) GetRoleTemplateById(ctx context.Context, id uuid.UUID) (*RoleTemplate, error) {
	query := `
		SELECT id, name, description, is_system, is_admin, created_at
		FROM role_templates
		WHERE id = $1
	`

	var t RoleTemplate
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&t.Id, &t.Name, &t.Description, &t.IsSystem, &t.IsAdmin, &t.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &t, nil
}

// GetRoleTemplatePermissions retrieves permission codes for a role template.
func (r *Repository) GetRoleTemplatePermissions(ctx context.Context, templateId uuid.UUID) ([]string, error) {
	query := `
		SELECT p.code
		FROM role_template_permissions rtp
		JOIN permissions p ON rtp.permission_id = p.id
		WHERE rtp.role_template_id = $1
		ORDER BY p.code
	`

	rows, err := r.db.QueryContext(ctx, query, templateId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		permissions = append(permissions, code)
	}

	return permissions, rows.Err()
}

// GetRoleTemplatesWithPermissions retrieves all role templates with their permissions.
func (r *Repository) GetRoleTemplatesWithPermissions(ctx context.Context) ([]RoleTemplateWithPermissions, error) {
	templates, err := r.GetAllRoleTemplates(ctx)
	if err != nil {
		return nil, err
	}

	var result []RoleTemplateWithPermissions
	for _, t := range templates {
		perms, err := r.GetRoleTemplatePermissions(ctx, t.Id)
		if err != nil {
			return nil, err
		}
		result = append(result, RoleTemplateWithPermissions{
			RoleTemplate: t,
			Permissions:  perms,
		})
	}

	return result, nil
}

// SetServerRolePermissions sets the permissions for a server role.
// This replaces all existing permissions.
func (r *Repository) SetServerRolePermissions(ctx context.Context, roleId uuid.UUID, permissionCodes []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing permissions
	_, err = tx.ExecContext(ctx, "DELETE FROM server_role_permissions WHERE server_role_id = $1", roleId)
	if err != nil {
		return err
	}

	// Insert new permissions
	for _, code := range permissionCodes {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO server_role_permissions (server_role_id, permission_id)
			SELECT $1, id FROM permissions WHERE code = $2
		`, roleId, code)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// AddServerRolePermission adds a single permission to a server role.
func (r *Repository) AddServerRolePermission(ctx context.Context, roleId uuid.UUID, permissionCode string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO server_role_permissions (server_role_id, permission_id)
		SELECT $1, id FROM permissions WHERE code = $2
		ON CONFLICT DO NOTHING
	`, roleId, permissionCode)
	return err
}

// RemoveServerRolePermission removes a single permission from a server role.
func (r *Repository) RemoveServerRolePermission(ctx context.Context, roleId uuid.UUID, permissionCode string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM server_role_permissions
		WHERE server_role_id = $1 AND permission_id = (SELECT id FROM permissions WHERE code = $2)
	`, roleId, permissionCode)
	return err
}

// GetServerRolePermissions retrieves permission codes for a server role.
func (r *Repository) GetServerRolePermissions(ctx context.Context, roleId uuid.UUID) ([]string, error) {
	query := `
		SELECT p.code
		FROM server_role_permissions srp
		JOIN permissions p ON srp.permission_id = p.id
		WHERE srp.server_role_id = $1
		ORDER BY p.code
	`

	rows, err := r.db.QueryContext(ctx, query, roleId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		permissions = append(permissions, code)
	}

	return permissions, rows.Err()
}

// CopyPermissionsFromTemplate copies permissions from a role template to a server role.
func (r *Repository) CopyPermissionsFromTemplate(ctx context.Context, roleId, templateId uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO server_role_permissions (server_role_id, permission_id)
		SELECT $1, permission_id FROM role_template_permissions WHERE role_template_id = $2
		ON CONFLICT DO NOTHING
	`, roleId, templateId)
	return err
}

// AddRoleInheritance creates an inheritance relationship between roles.
func (r *Repository) AddRoleInheritance(ctx context.Context, parentRoleId, childRoleId uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO role_inheritance (parent_role_id, child_role_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, parentRoleId, childRoleId)
	return err
}

// RemoveRoleInheritance removes an inheritance relationship between roles.
func (r *Repository) RemoveRoleInheritance(ctx context.Context, parentRoleId, childRoleId uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM role_inheritance
		WHERE parent_role_id = $1 AND child_role_id = $2
	`, parentRoleId, childRoleId)
	return err
}

// GetRoleParents retrieves the parent roles of a given role.
func (r *Repository) GetRoleParents(ctx context.Context, roleId uuid.UUID) ([]uuid.UUID, error) {
	query := `
		SELECT parent_role_id FROM role_inheritance WHERE child_role_id = $1
	`

	rows, err := r.db.QueryContext(ctx, query, roleId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var parents []uuid.UUID
	for rows.Next() {
		var parentId uuid.UUID
		if err := rows.Scan(&parentId); err != nil {
			return nil, err
		}
		parents = append(parents, parentId)
	}

	return parents, rows.Err()
}

// GetRoleChildren retrieves the child roles of a given role.
func (r *Repository) GetRoleChildren(ctx context.Context, roleId uuid.UUID) ([]uuid.UUID, error) {
	query := `
		SELECT child_role_id FROM role_inheritance WHERE parent_role_id = $1
	`

	rows, err := r.db.QueryContext(ctx, query, roleId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var children []uuid.UUID
	for rows.Next() {
		var childId uuid.UUID
		if err := rows.Scan(&childId); err != nil {
			return nil, err
		}
		children = append(children, childId)
	}

	return children, rows.Err()
}
