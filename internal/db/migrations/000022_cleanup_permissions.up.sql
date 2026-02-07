-- Migration 000022: Cleanup permission scopes
-- 1. Seed missing MOTD permissions (ui:motd:view, ui:motd:manage)
-- 2. Remove unused api: category permissions

-- =============================================================================
-- PHASE 1: Seed missing MOTD permissions
-- =============================================================================

INSERT INTO permissions (code, category, name, description, squad_permission) VALUES
    ('ui:motd:view', 'ui', 'View MOTD', 'Access to view MOTD configuration', NULL),
    ('ui:motd:manage', 'ui', 'Manage MOTD', 'Permission to manage MOTD settings and uploads', NULL)
ON CONFLICT (code) DO NOTHING;

-- Add MOTD permissions to Server Admin template
INSERT INTO role_template_permissions (role_template_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000002', id FROM permissions
WHERE code IN ('ui:motd:view', 'ui:motd:manage')
ON CONFLICT DO NOTHING;

-- Add MOTD view to Moderator template
INSERT INTO role_template_permissions (role_template_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000003', id FROM permissions
WHERE code = 'ui:motd:view'
ON CONFLICT DO NOTHING;

-- Backfill: grant MOTD permissions to existing server roles that have rcon:manageserver
INSERT INTO server_role_permissions (server_role_id, permission_id)
SELECT DISTINCT srp.server_role_id, p2.id
FROM server_role_permissions srp
JOIN permissions p1 ON srp.permission_id = p1.id AND p1.code = 'rcon:manageserver'
CROSS JOIN permissions p2
WHERE p2.code IN ('ui:motd:view', 'ui:motd:manage')
ON CONFLICT DO NOTHING;

-- =============================================================================
-- PHASE 2: Remove unused api: category permissions
-- =============================================================================

-- Remove api: permissions from server role assignments
DELETE FROM server_role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE category = 'api');

-- Remove api: permissions from role template assignments
DELETE FROM role_template_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE category = 'api');

-- Remove api: permission definitions
DELETE FROM permissions WHERE category = 'api';
