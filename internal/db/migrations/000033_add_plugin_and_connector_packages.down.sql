-- This drops only DB state. Native plugin and connector runtime files under
-- pluginRuntimeDir() and connectorRuntimeDir() are not removed; operators
-- rolling back must clean those directories manually for a fresh uninstall.

DROP INDEX IF EXISTS idx_server_admins_orphan_plugin_id;
DROP INDEX IF EXISTS idx_server_admins_server_id_managed_by_plugin_instance_id;
DROP INDEX IF EXISTS idx_server_admins_managed_by_plugin_instance_id;
ALTER TABLE server_admins
    DROP CONSTRAINT IF EXISTS fk_server_admins_managed_by_plugin_instance_id_plugin_instances_id;
ALTER TABLE server_admins
    DROP COLUMN IF EXISTS managed_by_plugin_id;
ALTER TABLE server_admins
    DROP COLUMN IF EXISTS managed_by_plugin_instance_id;

-- IF EXISTS keeps these idempotent for environments that never created the index.
DROP INDEX IF EXISTS idx_connector_packages_install_state;
DROP INDEX IF EXISTS idx_connector_packages_distribution;
DROP INDEX IF EXISTS idx_connector_packages_source;
DROP TABLE IF EXISTS connector_packages;

DROP INDEX IF EXISTS idx_plugin_packages_install_state;
DROP INDEX IF EXISTS idx_plugin_packages_distribution;
DROP INDEX IF EXISTS idx_plugin_packages_source;
DROP TABLE IF EXISTS plugin_packages;
