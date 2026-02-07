-- Reverse migration 000022: Restore api: permissions, remove MOTD permissions

-- =============================================================================
-- PHASE 1: Restore api: permission definitions
-- =============================================================================

INSERT INTO permissions (code, category, name, description, squad_permission) VALUES
    ('api:servers:read', 'api', 'Read Servers', 'API access to read server data', NULL),
    ('api:servers:write', 'api', 'Write Servers', 'API access to modify server settings', NULL),
    ('api:bans:read', 'api', 'Read Bans', 'API access to read bans', NULL),
    ('api:bans:write', 'api', 'Write Bans', 'API access to create/modify bans', NULL),
    ('api:players:read', 'api', 'Read Players', 'API access to read player data', NULL),
    ('api:rcon:execute', 'api', 'Execute RCON', 'API access to execute RCON commands', NULL),
    ('api:plugins:manage', 'api', 'Manage Plugins', 'API access to manage plugins', NULL),
    ('api:workflows:manage', 'api', 'Manage Workflows', 'API access to manage workflows', NULL),
    ('api:rules:manage', 'api', 'Manage Rules', 'API access to manage server rules', NULL),
    ('api:evidence:upload', 'api', 'Upload Evidence', 'API access to upload evidence files', NULL),
    ('api:evidence:read', 'api', 'Read Evidence', 'API access to read evidence files', NULL)
ON CONFLICT (code) DO NOTHING;

-- Restore api: permissions to Server Admin template
INSERT INTO role_template_permissions (role_template_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000002', id FROM permissions
WHERE code IN (
    'api:servers:read', 'api:servers:write', 'api:bans:read', 'api:bans:write',
    'api:players:read', 'api:rcon:execute', 'api:plugins:manage', 'api:workflows:manage',
    'api:rules:manage', 'api:evidence:upload', 'api:evidence:read'
)
ON CONFLICT DO NOTHING;

-- Restore api: permissions to Moderator template
INSERT INTO role_template_permissions (role_template_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000003', id FROM permissions
WHERE code IN ('api:servers:read', 'api:bans:read', 'api:bans:write', 'api:players:read',
               'api:evidence:upload', 'api:evidence:read')
ON CONFLICT DO NOTHING;

-- Restore api: permissions to Junior Moderator template
INSERT INTO role_template_permissions (role_template_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000004', id FROM permissions
WHERE code IN ('api:servers:read', 'api:players:read')
ON CONFLICT DO NOTHING;

-- =============================================================================
-- PHASE 2: Remove MOTD permissions
-- =============================================================================

-- Remove MOTD permissions from server role assignments
DELETE FROM server_role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE code IN ('ui:motd:view', 'ui:motd:manage'));

-- Remove MOTD permissions from role template assignments
DELETE FROM role_template_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE code IN ('ui:motd:view', 'ui:motd:manage'));

-- Remove MOTD permission definitions
DELETE FROM permissions WHERE code IN ('ui:motd:view', 'ui:motd:manage');
