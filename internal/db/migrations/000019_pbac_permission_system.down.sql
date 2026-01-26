-- Rollback PBAC Permission System Migration

-- Drop tables in reverse order (respecting foreign key constraints)
DROP TABLE IF EXISTS role_inheritance;
DROP TABLE IF EXISTS server_role_permissions;
DROP TABLE IF EXISTS role_template_permissions;
DROP TABLE IF EXISTS role_templates;
DROP TABLE IF EXISTS permissions;

-- Remove deprecation comment from server_roles.permissions
COMMENT ON COLUMN server_roles.permissions IS NULL;
