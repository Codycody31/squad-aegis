-- Remove is_admin flag from server_roles table
DROP INDEX IF EXISTS idx_server_roles_is_admin;
ALTER TABLE server_roles DROP COLUMN IF EXISTS is_admin;
