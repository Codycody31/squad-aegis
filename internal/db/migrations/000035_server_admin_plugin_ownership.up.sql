ALTER TABLE server_admins
    ADD COLUMN managed_by_plugin_instance_id UUID;

ALTER TABLE server_admins
    ADD CONSTRAINT fk_server_admins_managed_by_plugin_instance_id_plugin_instances_id
    FOREIGN KEY (managed_by_plugin_instance_id) REFERENCES plugin_instances(id) ON DELETE CASCADE;

CREATE INDEX idx_server_admins_managed_by_plugin_instance_id
    ON server_admins(managed_by_plugin_instance_id);

CREATE INDEX idx_server_admins_server_id_managed_by_plugin_instance_id
    ON server_admins(server_id, managed_by_plugin_instance_id);

WITH candidate_plugin_instances AS (
    SELECT DISTINCT ON (server_id, plugin_id)
        id,
        server_id,
        plugin_id
    FROM plugin_instances
    WHERE plugin_id IN ('server_seeder_whitelist', 'squad_leader_whitelist')
    ORDER BY server_id, plugin_id, updated_at DESC, created_at DESC, id
)
UPDATE server_admins sa
SET managed_by_plugin_instance_id = cpi.id
FROM candidate_plugin_instances cpi
WHERE sa.managed_by_plugin_instance_id IS NULL
  AND sa.server_id = cpi.server_id
  AND (
      (cpi.plugin_id = 'server_seeder_whitelist' AND sa.notes LIKE 'Plugin: Seeder Whitelist - %')
      OR
      (cpi.plugin_id = 'squad_leader_whitelist' AND sa.notes LIKE 'Plugin: Squad Leader Whitelist - %')
  );
