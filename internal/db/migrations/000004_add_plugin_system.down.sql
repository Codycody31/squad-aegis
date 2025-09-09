-- Remove plugin system tables

-- Drop indexes first
DROP INDEX IF EXISTS idx_connectors_enabled;
DROP INDEX IF EXISTS idx_plugin_data_plugin_instance_id;
DROP INDEX IF EXISTS idx_plugin_instances_enabled;
DROP INDEX IF EXISTS idx_plugin_instances_plugin_id;
DROP INDEX IF EXISTS idx_plugin_instances_server_id;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS plugin_data;
DROP TABLE IF EXISTS plugin_instances;
DROP TABLE IF EXISTS connectors;
