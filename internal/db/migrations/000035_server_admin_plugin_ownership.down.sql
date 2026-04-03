DROP INDEX IF EXISTS idx_server_admins_server_id_managed_by_plugin_instance_id;

DROP INDEX IF EXISTS idx_server_admins_managed_by_plugin_instance_id;

ALTER TABLE server_admins
    DROP CONSTRAINT IF EXISTS fk_server_admins_managed_by_plugin_instance_id_plugin_instances_id;

ALTER TABLE server_admins
    DROP COLUMN IF EXISTS managed_by_plugin_instance_id;
