-- Migration 000030: Add dedicated UI permission for map changes
-- This allows roles to use the dashboard map-change UI without granting
-- generic console execution access.

INSERT INTO permissions (code, category, name, description, squad_permission) VALUES
    ('ui:maps:change', 'ui', 'Change Maps', 'Permission to change the current layer or set the next layer from the UI', NULL)
ON CONFLICT (code) DO NOTHING;

-- Backfill role templates that already have in-game map change access.
INSERT INTO role_template_permissions (role_template_id, permission_id)
SELECT DISTINCT existing.role_template_id, new_perm.id
FROM role_template_permissions existing
JOIN permissions existing_perm ON existing.permission_id = existing_perm.id
JOIN permissions new_perm ON new_perm.code = 'ui:maps:change'
WHERE existing_perm.code = 'rcon:changemap'
ON CONFLICT DO NOTHING;

-- Backfill existing server roles that already have in-game map change access.
INSERT INTO server_role_permissions (server_role_id, permission_id)
SELECT DISTINCT existing.server_role_id, new_perm.id
FROM server_role_permissions existing
JOIN permissions existing_perm ON existing.permission_id = existing_perm.id
JOIN permissions new_perm ON new_perm.code = 'ui:maps:change'
WHERE existing_perm.code = 'rcon:changemap'
ON CONFLICT DO NOTHING;
