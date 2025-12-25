-- Drop custom plugin system tables and columns

DROP INDEX IF EXISTS idx_server_plugins_plugin_source;
ALTER TABLE server_plugins DROP COLUMN IF EXISTS plugin_source;

DROP INDEX IF EXISTS idx_plugin_sandbox_configs_plugin_instance_id;
DROP TABLE IF EXISTS plugin_sandbox_configs;

DROP INDEX IF EXISTS idx_plugin_public_keys_revoked;
DROP INDEX IF EXISTS idx_plugin_public_keys_key_name;
DROP TABLE IF EXISTS plugin_public_keys;

DROP INDEX IF EXISTS idx_plugin_permissions_permission_id;
DROP INDEX IF EXISTS idx_plugin_permissions_plugin_id;
DROP TABLE IF EXISTS plugin_permissions;

DROP INDEX IF EXISTS idx_custom_plugins_verified;
DROP INDEX IF EXISTS idx_custom_plugins_enabled;
DROP INDEX IF EXISTS idx_custom_plugins_uploaded_by;
DROP INDEX IF EXISTS idx_custom_plugins_plugin_id;
DROP TABLE IF EXISTS custom_plugins;

