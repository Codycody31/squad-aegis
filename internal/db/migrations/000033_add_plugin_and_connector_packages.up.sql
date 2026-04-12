-- plugin_packages: stores installed native plugin package metadata
CREATE TABLE IF NOT EXISTS plugin_packages (
    plugin_id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'native',
    distribution TEXT NOT NULL DEFAULT 'sideload',
    official BOOLEAN NOT NULL DEFAULT false,
    install_state TEXT NOT NULL DEFAULT 'ready',
    runtime_path TEXT NOT NULL DEFAULT '',
    manifest_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    manifest_signature TEXT NOT NULL DEFAULT '',
    manifest_public_key TEXT NOT NULL DEFAULT '',
    signature_verified BOOLEAN NOT NULL DEFAULT false,
    unsafe BOOLEAN NOT NULL DEFAULT false,
    checksum TEXT NOT NULL DEFAULT '',
    min_host_api_version INTEGER NOT NULL DEFAULT 0,
    required_capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
    target_os TEXT NOT NULL DEFAULT '',
    target_arch TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_plugin_packages_source ON plugin_packages(source);
CREATE INDEX IF NOT EXISTS idx_plugin_packages_distribution ON plugin_packages(distribution);
CREATE INDEX IF NOT EXISTS idx_plugin_packages_install_state ON plugin_packages(install_state);

-- connector_packages: stores installed native connector package metadata
CREATE TABLE IF NOT EXISTS connector_packages (
    connector_id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'native',
    distribution TEXT NOT NULL DEFAULT 'sideload',
    official BOOLEAN NOT NULL DEFAULT false,
    install_state TEXT NOT NULL DEFAULT 'ready',
    runtime_path TEXT NOT NULL DEFAULT '',
    manifest_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    manifest_signature TEXT NOT NULL DEFAULT '',
    manifest_public_key TEXT NOT NULL DEFAULT '',
    signature_verified BOOLEAN NOT NULL DEFAULT false,
    unsafe BOOLEAN NOT NULL DEFAULT false,
    checksum TEXT NOT NULL DEFAULT '',
    min_host_api_version INTEGER NOT NULL DEFAULT 0,
    required_capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
    target_os TEXT NOT NULL DEFAULT '',
    target_arch TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_connector_packages_source ON connector_packages(source);
CREATE INDEX IF NOT EXISTS idx_connector_packages_install_state ON connector_packages(install_state);

-- server_admins: track which plugin instance manages each admin row
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
