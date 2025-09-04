CREATE TABLE IF NOT EXISTS plugin_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    plugin_id TEXT NOT NULL,
    name TEXT NOT NULL,
    config JSONB NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(server_id, plugin_id, name)
);

CREATE TABLE IF NOT EXISTS plugin_data (
    plugin_instance_id UUID NOT NULL REFERENCES plugin_instances(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (plugin_instance_id, key)
);

CREATE TABLE IF NOT EXISTS connectors (
    id TEXT PRIMARY KEY,
    config JSONB NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_plugin_instances_server_id ON plugin_instances(server_id);
CREATE INDEX IF NOT EXISTS idx_plugin_instances_plugin_id ON plugin_instances(plugin_id);
CREATE INDEX IF NOT EXISTS idx_plugin_instances_enabled ON plugin_instances(enabled);
CREATE INDEX IF NOT EXISTS idx_plugin_data_plugin_instance_id ON plugin_data(plugin_instance_id);
CREATE INDEX IF NOT EXISTS idx_connectors_enabled ON connectors(enabled);