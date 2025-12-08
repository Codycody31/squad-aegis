-- Add is_admin flag to server_roles table
-- This flag distinguishes between true admin roles (should receive pings, be in admin lists)
-- and access-only roles (like "reserved" which just provides quicker server access)
ALTER TABLE server_roles ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT true;

-- Create index for filtering admin roles
CREATE INDEX idx_server_roles_is_admin ON server_roles(is_admin) WHERE is_admin = true;
