DELETE FROM server_role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE code = 'ui:maps:change');

DELETE FROM role_template_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE code = 'ui:maps:change');

DELETE FROM permissions WHERE code = 'ui:maps:change';
