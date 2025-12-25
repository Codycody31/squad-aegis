-- Custom plugins table for user-uploaded .so plugins
CREATE TABLE IF NOT EXISTS custom_plugins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_id VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(50) NOT NULL,
    author VARCHAR(255),
    sdk_version VARCHAR(20) NOT NULL,
    description TEXT,
    website VARCHAR(500),
    storage_path TEXT NOT NULL,
    signature BYTEA,
    uploaded_by UUID REFERENCES users(id),
    uploaded_at TIMESTAMP DEFAULT NOW(),
    required_features TEXT[] DEFAULT '{}',
    provided_features TEXT[] DEFAULT '{}',
    required_permissions TEXT[] DEFAULT '{}',
    allow_multiple_instances BOOLEAN DEFAULT FALSE,
    long_running BOOLEAN DEFAULT FALSE,
    enabled BOOLEAN DEFAULT FALSE,
    verified BOOLEAN DEFAULT FALSE,
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_custom_plugins_plugin_id ON custom_plugins(plugin_id);
CREATE INDEX idx_custom_plugins_uploaded_by ON custom_plugins(uploaded_by);
CREATE INDEX idx_custom_plugins_enabled ON custom_plugins(enabled);
CREATE INDEX idx_custom_plugins_verified ON custom_plugins(verified);

-- Plugin permissions table for granular permission control
CREATE TABLE IF NOT EXISTS plugin_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_id VARCHAR(255) NOT NULL,
    permission_id VARCHAR(100) NOT NULL,
    granted_by UUID REFERENCES users(id),
    granted_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(plugin_id, permission_id)
);

CREATE INDEX idx_plugin_permissions_plugin_id ON plugin_permissions(plugin_id);
CREATE INDEX idx_plugin_permissions_permission_id ON plugin_permissions(permission_id);

-- Public keys table for plugin signature verification
CREATE TABLE IF NOT EXISTS plugin_public_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_name VARCHAR(255) UNIQUE NOT NULL,
    public_key BYTEA NOT NULL,
    algorithm VARCHAR(50) NOT NULL DEFAULT 'ed25519',
    added_by UUID REFERENCES users(id),
    added_at TIMESTAMP DEFAULT NOW(),
    revoked BOOLEAN DEFAULT FALSE,
    revoked_at TIMESTAMP,
    revoked_by UUID REFERENCES users(id)
);

CREATE INDEX idx_plugin_public_keys_key_name ON plugin_public_keys(key_name);
CREATE INDEX idx_plugin_public_keys_revoked ON plugin_public_keys(revoked);

-- Plugin sandbox configuration table (optional per-instance resource limits)
CREATE TABLE IF NOT EXISTS plugin_sandbox_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_instance_id UUID REFERENCES server_plugins(id) ON DELETE CASCADE,
    max_memory_mb INTEGER DEFAULT 512,
    max_goroutines INTEGER DEFAULT 100,
    cpu_time_limit_seconds INTEGER DEFAULT 0,
    enable_monitoring BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(plugin_instance_id)
);

CREATE INDEX idx_plugin_sandbox_configs_plugin_instance_id ON plugin_sandbox_configs(plugin_instance_id);

-- Add plugin_source column to server_plugins to distinguish builtin vs custom
ALTER TABLE server_plugins ADD COLUMN IF NOT EXISTS plugin_source VARCHAR(20) DEFAULT 'builtin';
CREATE INDEX idx_server_plugins_plugin_source ON server_plugins(plugin_source);

-- Add comments for documentation
COMMENT ON TABLE custom_plugins IS 'Stores metadata for user-uploaded custom plugins (.so files)';
COMMENT ON TABLE plugin_permissions IS 'Manages granular permissions granted to plugins';
COMMENT ON TABLE plugin_public_keys IS 'Stores trusted public keys for plugin signature verification';
COMMENT ON TABLE plugin_sandbox_configs IS 'Per-instance resource limits for plugin sandboxing';

COMMENT ON COLUMN custom_plugins.plugin_id IS 'Unique identifier for the plugin (used in code)';
COMMENT ON COLUMN custom_plugins.sdk_version IS 'Version of the plugin SDK this plugin was built against';
COMMENT ON COLUMN custom_plugins.storage_path IS 'Path to the .so file in the storage backend';
COMMENT ON COLUMN custom_plugins.signature IS 'Cryptographic signature of the plugin binary';
COMMENT ON COLUMN custom_plugins.required_features IS 'Array of feature IDs this plugin requires';
COMMENT ON COLUMN custom_plugins.provided_features IS 'Array of feature IDs this plugin provides';
COMMENT ON COLUMN custom_plugins.required_permissions IS 'Array of permission IDs this plugin requires';
COMMENT ON COLUMN custom_plugins.verified IS 'Whether the plugin signature has been verified';

COMMENT ON COLUMN plugin_permissions.plugin_id IS 'Plugin ID (not FK to allow both builtin and custom)';
COMMENT ON COLUMN plugin_permissions.permission_id IS 'Permission identifier (e.g., rcon.access, database.read)';

COMMENT ON COLUMN plugin_public_keys.algorithm IS 'Signature algorithm (currently only ed25519 supported)';
COMMENT ON COLUMN plugin_public_keys.revoked IS 'Whether this key has been revoked';

