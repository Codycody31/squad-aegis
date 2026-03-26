INSERT INTO permissions (code, category, name, description, squad_permission) VALUES
    ('ui:maps:change', 'ui', 'Change Maps', 'Permission to change the current layer or set the next layer from the UI', NULL)
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_template_permissions (role_template_id, permission_id)
SELECT DISTINCT existing.role_template_id, new_perm.id
FROM role_template_permissions existing
JOIN permissions existing_perm ON existing.permission_id = existing_perm.id
JOIN permissions new_perm ON new_perm.code = 'ui:maps:change'
WHERE existing_perm.code = 'rcon:changemap'
ON CONFLICT DO NOTHING;

INSERT INTO server_role_permissions (server_role_id, permission_id)
SELECT DISTINCT existing.server_role_id, new_perm.id
FROM server_role_permissions existing
JOIN permissions existing_perm ON existing.permission_id = existing_perm.id
JOIN permissions new_perm ON new_perm.code = 'ui:maps:change'
WHERE existing_perm.code = 'rcon:changemap'
ON CONFLICT DO NOTHING;
