-- PBAC Permission System Migration
-- This migration introduces a proper Policy-Based Access Control system with:
-- 1. Normalized permission definitions
-- 2. Role templates for predefined roles
-- 3. Server role to permission mapping (replacing comma-separated string)
-- 4. Role inheritance support

-- =============================================================================
-- PHASE 1: Create new tables
-- =============================================================================

-- Permission definitions table
CREATE TABLE permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(100) UNIQUE NOT NULL,
    category VARCHAR(50) NOT NULL,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    squad_permission VARCHAR(50),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_permissions_category ON permissions(category);
CREATE INDEX idx_permissions_code ON permissions(code);

-- Role templates (global predefined roles)
CREATE TABLE role_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    is_admin BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Role template to permission mapping
CREATE TABLE role_template_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_template_id UUID NOT NULL REFERENCES role_templates(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    UNIQUE(role_template_id, permission_id)
);

CREATE INDEX idx_role_template_permissions_template ON role_template_permissions(role_template_id);
CREATE INDEX idx_role_template_permissions_permission ON role_template_permissions(permission_id);

-- Server role to permission mapping (replaces comma-separated permissions column)
CREATE TABLE server_role_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_role_id UUID NOT NULL REFERENCES server_roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    UNIQUE(server_role_id, permission_id)
);

CREATE INDEX idx_server_role_permissions_role ON server_role_permissions(server_role_id);
CREATE INDEX idx_server_role_permissions_permission ON server_role_permissions(permission_id);

-- Role inheritance (for role hierarchy)
CREATE TABLE role_inheritance (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_role_id UUID NOT NULL REFERENCES server_roles(id) ON DELETE CASCADE,
    child_role_id UUID NOT NULL REFERENCES server_roles(id) ON DELETE CASCADE,
    UNIQUE(parent_role_id, child_role_id),
    CHECK(parent_role_id != child_role_id)
);

CREATE INDEX idx_role_inheritance_parent ON role_inheritance(parent_role_id);
CREATE INDEX idx_role_inheritance_child ON role_inheritance(child_role_id);

-- =============================================================================
-- PHASE 2: Seed permission definitions
-- =============================================================================

-- Wildcard permission
INSERT INTO permissions (code, category, name, description, squad_permission) VALUES
    ('*', 'system', 'All Permissions', 'Grants all permissions (wildcard)', NULL);

-- UI Permissions
INSERT INTO permissions (code, category, name, description, squad_permission) VALUES
    ('ui:dashboard:view', 'ui', 'View Dashboard', 'Access to view the server dashboard', NULL),
    ('ui:audit_logs:view', 'ui', 'View Audit Logs', 'Access to view audit logs', NULL),
    ('ui:metrics:view', 'ui', 'View Metrics', 'Access to view server metrics', NULL),
    ('ui:feeds:view', 'ui', 'View Feeds', 'Access to view live feeds (chat, connections)', NULL),
    ('ui:console:view', 'ui', 'View Console', 'Access to view RCON console', NULL),
    ('ui:console:execute', 'ui', 'Execute Console Commands', 'Permission to execute RCON commands from console', NULL),
    ('ui:plugins:view', 'ui', 'View Plugins', 'Access to view plugins', NULL),
    ('ui:plugins:manage', 'ui', 'Manage Plugins', 'Permission to enable/disable and configure plugins', NULL),
    ('ui:workflows:view', 'ui', 'View Workflows', 'Access to view workflows', NULL),
    ('ui:workflows:manage', 'ui', 'Manage Workflows', 'Permission to create, edit, and delete workflows', NULL),
    ('ui:settings:view', 'ui', 'View Settings', 'Access to view server settings', NULL),
    ('ui:settings:manage', 'ui', 'Manage Settings', 'Permission to modify server settings', NULL),
    ('ui:users:manage', 'ui', 'Manage Users', 'Permission to manage users and admins', NULL),
    ('ui:roles:manage', 'ui', 'Manage Roles', 'Permission to manage roles and permissions', NULL),
    ('ui:bans:view', 'ui', 'View Bans', 'Permission to view ban list', NULL),
    ('ui:bans:create', 'ui', 'Create Bans', 'Permission to create bans', NULL),
    ('ui:bans:edit', 'ui', 'Edit Bans', 'Permission to edit existing bans', NULL),
    ('ui:bans:delete', 'ui', 'Delete Bans', 'Permission to delete bans', NULL),
    ('ui:players:view', 'ui', 'View Players', 'Permission to view player list', NULL),
    ('ui:players:kick', 'ui', 'Kick Players', 'Permission to kick players from UI', NULL),
    ('ui:players:warn', 'ui', 'Warn Players', 'Permission to warn players', NULL),
    ('ui:players:move', 'ui', 'Move Players', 'Permission to move players between teams', NULL),
    ('ui:rules:view', 'ui', 'View Rules', 'Access to view server rules', NULL),
    ('ui:rules:manage', 'ui', 'Manage Rules', 'Permission to create, edit, and delete server rules', NULL),
    ('ui:ban_lists:view', 'ui', 'View Ban Lists', 'Access to view ban lists', NULL),
    ('ui:ban_lists:manage', 'ui', 'Manage Ban Lists', 'Permission to manage ban list subscriptions', NULL);

-- API Permissions
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
    ('api:evidence:read', 'api', 'Read Evidence', 'API access to read evidence files', NULL);

-- RCON/Squad Permissions (maps to Squad admin.cfg format)
INSERT INTO permissions (code, category, name, description, squad_permission) VALUES
    ('rcon:reserve', 'rcon', 'Reserve Slot', 'Reserved slot access on the server', 'reserve'),
    ('rcon:balance', 'rcon', 'Balance Teams', 'Permission to balance teams', 'balance'),
    ('rcon:canseeadminchat', 'rcon', 'See Admin Chat', 'Permission to see admin chat', 'canseeadminchat'),
    ('rcon:manageserver', 'rcon', 'Manage Server', 'Full server management permission', 'manageserver'),
    ('rcon:teamchange', 'rcon', 'Change Team', 'Permission to change own team', 'teamchange'),
    ('rcon:chat', 'rcon', 'Admin Chat', 'Permission to use admin chat', 'chat'),
    ('rcon:cameraman', 'rcon', 'Cameraman', 'Permission to use cameraman mode', 'cameraman'),
    ('rcon:kick', 'rcon', 'Kick Players', 'Permission to kick players', 'kick'),
    ('rcon:ban', 'rcon', 'Ban Players', 'Permission to ban players', 'ban'),
    ('rcon:forceteamchange', 'rcon', 'Force Team Change', 'Permission to force team changes', 'forceteamchange'),
    ('rcon:immune', 'rcon', 'Immune', 'Immunity from kicks/bans', 'immune'),
    ('rcon:changemap', 'rcon', 'Change Map', 'Permission to change the map', 'changemap'),
    ('rcon:pause', 'rcon', 'Pause Game', 'Permission to pause the game', 'pause'),
    ('rcon:cheat', 'rcon', 'Cheat Mode', 'Permission to use cheat commands', 'cheat'),
    ('rcon:private', 'rcon', 'Private Server', 'Permission to make server private', 'private'),
    ('rcon:config', 'rcon', 'Server Config', 'Permission to change server config', 'config'),
    ('rcon:featuretest', 'rcon', 'Feature Test', 'Permission to use feature test commands', 'featuretest'),
    ('rcon:demos', 'rcon', 'Record Demos', 'Permission to record demos', 'demos'),
    ('rcon:disbandsquad', 'rcon', 'Disband Squad', 'Permission to disband squads', 'disbandSquad'),
    ('rcon:removefromsquad', 'rcon', 'Remove From Squad', 'Permission to remove players from squads', 'removeFromSquad'),
    ('rcon:demotecommander', 'rcon', 'Demote Commander', 'Permission to demote commanders', 'demoteCommander'),
    ('rcon:debug', 'rcon', 'Debug Mode', 'Permission to use debug commands', 'debug');

-- =============================================================================
-- PHASE 3: Seed role templates
-- =============================================================================

-- Super Admin (wildcard)
INSERT INTO role_templates (id, name, description, is_system, is_admin) VALUES
    ('00000000-0000-0000-0000-000000000001', 'Super Admin', 'Full access to all features', TRUE, TRUE);

INSERT INTO role_template_permissions (role_template_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000001', id FROM permissions WHERE code = '*';

-- Server Admin
INSERT INTO role_templates (id, name, description, is_system, is_admin) VALUES
    ('00000000-0000-0000-0000-000000000002', 'Server Admin', 'Full server management including bans, kicks, and settings', TRUE, TRUE);

INSERT INTO role_template_permissions (role_template_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000002', id FROM permissions
WHERE code IN (
    -- All UI permissions
    'ui:dashboard:view', 'ui:audit_logs:view', 'ui:metrics:view', 'ui:feeds:view',
    'ui:console:view', 'ui:console:execute', 'ui:plugins:view', 'ui:plugins:manage',
    'ui:workflows:view', 'ui:workflows:manage', 'ui:settings:view', 'ui:settings:manage',
    'ui:users:manage', 'ui:roles:manage', 'ui:bans:view', 'ui:bans:create', 'ui:bans:edit', 'ui:bans:delete',
    'ui:players:view', 'ui:players:kick', 'ui:players:warn', 'ui:players:move',
    'ui:rules:view', 'ui:rules:manage', 'ui:ban_lists:view', 'ui:ban_lists:manage',
    -- All API permissions
    'api:servers:read', 'api:servers:write', 'api:bans:read', 'api:bans:write',
    'api:players:read', 'api:rcon:execute', 'api:plugins:manage', 'api:workflows:manage',
    'api:rules:manage', 'api:evidence:upload', 'api:evidence:read',
    -- Most RCON permissions (excluding cheat, debug, featuretest)
    'rcon:reserve', 'rcon:balance', 'rcon:canseeadminchat', 'rcon:manageserver',
    'rcon:teamchange', 'rcon:chat', 'rcon:cameraman', 'rcon:kick', 'rcon:ban',
    'rcon:forceteamchange', 'rcon:immune', 'rcon:changemap', 'rcon:pause',
    'rcon:private', 'rcon:config', 'rcon:demos', 'rcon:disbandsquad',
    'rcon:removefromsquad', 'rcon:demotecommander'
);

-- Moderator
INSERT INTO role_templates (id, name, description, is_system, is_admin) VALUES
    ('00000000-0000-0000-0000-000000000003', 'Moderator', 'Player management including kicks, warns, and bans', TRUE, TRUE);

INSERT INTO role_template_permissions (role_template_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000003', id FROM permissions
WHERE code IN (
    'ui:dashboard:view', 'ui:feeds:view', 'ui:bans:view', 'ui:bans:create', 'ui:bans:edit',
    'ui:players:view', 'ui:players:kick', 'ui:players:warn', 'ui:players:move',
    'ui:rules:view', 'ui:console:view',
    'api:servers:read', 'api:bans:read', 'api:bans:write', 'api:players:read',
    'api:evidence:upload', 'api:evidence:read',
    'rcon:reserve', 'rcon:balance', 'rcon:canseeadminchat', 'rcon:teamchange',
    'rcon:chat', 'rcon:kick', 'rcon:ban', 'rcon:forceteamchange',
    'rcon:disbandsquad', 'rcon:removefromsquad'
);

-- Junior Moderator
INSERT INTO role_templates (id, name, description, is_system, is_admin) VALUES
    ('00000000-0000-0000-0000-000000000004', 'Junior Moderator', 'Limited moderation - kicks and warns only', TRUE, TRUE);

INSERT INTO role_template_permissions (role_template_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000004', id FROM permissions
WHERE code IN (
    'ui:dashboard:view', 'ui:feeds:view', 'ui:players:view', 'ui:players:kick', 'ui:players:warn',
    'ui:rules:view',
    'api:servers:read', 'api:players:read',
    'rcon:reserve', 'rcon:canseeadminchat', 'rcon:teamchange', 'rcon:chat', 'rcon:kick'
);

-- Reserved Player (whitelist - is_admin = false)
INSERT INTO role_templates (id, name, description, is_system, is_admin) VALUES
    ('00000000-0000-0000-0000-000000000005', 'Reserved Player', 'Reserved slot access only (whitelist)', TRUE, FALSE);

INSERT INTO role_template_permissions (role_template_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000005', id FROM permissions
WHERE code IN ('rcon:reserve');

-- Streamer (whitelist - is_admin = false)
INSERT INTO role_templates (id, name, description, is_system, is_admin) VALUES
    ('00000000-0000-0000-0000-000000000006', 'Streamer', 'Reserved slot and cameraman mode for content creators', TRUE, FALSE);

INSERT INTO role_template_permissions (role_template_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000006', id FROM permissions
WHERE code IN ('rcon:reserve', 'rcon:cameraman');

-- VIP (whitelist - is_admin = false)
INSERT INTO role_templates (id, name, description, is_system, is_admin) VALUES
    ('00000000-0000-0000-0000-000000000007', 'VIP', 'Reserved slot for donors and supporters', TRUE, FALSE);

INSERT INTO role_template_permissions (role_template_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000007', id FROM permissions
WHERE code IN ('rcon:reserve');

-- =============================================================================
-- PHASE 4: Migrate existing server_roles data
-- =============================================================================

-- Create function to migrate permissions
CREATE OR REPLACE FUNCTION migrate_role_permissions() RETURNS void AS $$
DECLARE
    role_record RECORD;
    perm_code TEXT;
    perm_id UUID;
    old_perm TEXT;
BEGIN
    -- Loop through all existing server roles
    FOR role_record IN SELECT id, permissions, is_admin FROM server_roles LOOP
        -- Parse comma-separated permissions
        FOREACH old_perm IN ARRAY string_to_array(role_record.permissions, ',') LOOP
            old_perm := trim(old_perm);

            -- Skip empty strings
            IF old_perm = '' THEN
                CONTINUE;
            END IF;

            -- Map old permission names to new permission codes
            CASE old_perm
                -- Wildcard
                WHEN '*' THEN perm_id := (SELECT id FROM permissions WHERE code = '*');

                -- RCON permissions (direct mapping)
                WHEN 'reserve' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:reserve');
                WHEN 'balance' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:balance');
                WHEN 'canseeadminchat' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:canseeadminchat');
                WHEN 'manageserver' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:manageserver');
                WHEN 'teamchange' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:teamchange');
                WHEN 'chat' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:chat');
                WHEN 'cameraman' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:cameraman');
                WHEN 'kick' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:kick');
                WHEN 'ban' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:ban');
                WHEN 'forceteamchange' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:forceteamchange');
                WHEN 'immune' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:immune');
                WHEN 'changemap' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:changemap');
                WHEN 'pause' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:pause');
                WHEN 'cheat' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:cheat');
                WHEN 'private' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:private');
                WHEN 'config' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:config');
                WHEN 'featuretest' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:featuretest');
                WHEN 'demos' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:demos');
                WHEN 'disbandSquad' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:disbandsquad');
                WHEN 'removeFromSquad' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:removefromsquad');
                WHEN 'demoteCommander' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:demotecommander');
                WHEN 'debug' THEN perm_id := (SELECT id FROM permissions WHERE code = 'rcon:debug');
                ELSE perm_id := NULL;
            END CASE;

            -- Insert the permission if found
            IF perm_id IS NOT NULL THEN
                INSERT INTO server_role_permissions (server_role_id, permission_id)
                VALUES (role_record.id, perm_id)
                ON CONFLICT DO NOTHING;
            END IF;
        END LOOP;

        -- Grant additional UI/API permissions based on old RCON permissions (only for admin roles)
        IF role_record.is_admin THEN
            -- If role has wildcard, don't add individual permissions
            IF EXISTS (SELECT 1 FROM server_role_permissions srp
                       JOIN permissions p ON srp.permission_id = p.id
                       WHERE srp.server_role_id = role_record.id AND p.code = '*') THEN
                CONTINUE;
            END IF;

            -- If role has ban permission, grant ban-related UI/API permissions
            IF EXISTS (SELECT 1 FROM server_role_permissions srp
                       JOIN permissions p ON srp.permission_id = p.id
                       WHERE srp.server_role_id = role_record.id AND p.code = 'rcon:ban') THEN
                INSERT INTO server_role_permissions (server_role_id, permission_id)
                SELECT role_record.id, id FROM permissions
                WHERE code IN ('ui:bans:view', 'ui:bans:create', 'ui:bans:edit', 'api:bans:read', 'api:bans:write', 'api:evidence:upload', 'api:evidence:read')
                ON CONFLICT DO NOTHING;
            END IF;

            -- If role has kick permission, grant kick-related UI permissions
            IF EXISTS (SELECT 1 FROM server_role_permissions srp
                       JOIN permissions p ON srp.permission_id = p.id
                       WHERE srp.server_role_id = role_record.id AND p.code = 'rcon:kick') THEN
                INSERT INTO server_role_permissions (server_role_id, permission_id)
                SELECT role_record.id, id FROM permissions
                WHERE code IN ('ui:players:view', 'ui:players:kick', 'ui:players:warn', 'api:players:read')
                ON CONFLICT DO NOTHING;
            END IF;

            -- If role has manageserver permission, grant full UI/API access
            IF EXISTS (SELECT 1 FROM server_role_permissions srp
                       JOIN permissions p ON srp.permission_id = p.id
                       WHERE srp.server_role_id = role_record.id AND p.code = 'rcon:manageserver') THEN
                INSERT INTO server_role_permissions (server_role_id, permission_id)
                SELECT role_record.id, id FROM permissions
                WHERE category IN ('ui', 'api')
                ON CONFLICT DO NOTHING;
            END IF;

            -- Grant basic view permissions to all admin roles
            INSERT INTO server_role_permissions (server_role_id, permission_id)
            SELECT role_record.id, id FROM permissions
            WHERE code IN ('ui:dashboard:view', 'ui:players:view', 'ui:feeds:view', 'api:servers:read', 'api:players:read')
            ON CONFLICT DO NOTHING;
        END IF;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Execute migration
SELECT migrate_role_permissions();

-- Drop the migration function
DROP FUNCTION migrate_role_permissions();

-- =============================================================================
-- PHASE 5: Mark old column as deprecated (keep for backward compatibility)
-- =============================================================================

COMMENT ON COLUMN server_roles.permissions IS 'DEPRECATED: Use server_role_permissions table instead. This column is kept for backward compatibility and will be removed in a future version.';
