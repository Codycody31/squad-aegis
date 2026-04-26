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
    signed_manifest_json TEXT NOT NULL DEFAULT '',
    signature_key_id TEXT NOT NULL DEFAULT '',
    signature_signed_at TIMESTAMP WITH TIME ZONE,
    signature_expires_at TIMESTAMP WITH TIME ZONE,
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
    signed_manifest_json TEXT NOT NULL DEFAULT '',
    signature_key_id TEXT NOT NULL DEFAULT '',
    signature_signed_at TIMESTAMP WITH TIME ZONE,
    signature_expires_at TIMESTAMP WITH TIME ZONE,
    min_host_api_version INTEGER NOT NULL DEFAULT 0,
    required_capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
    target_os TEXT NOT NULL DEFAULT '',
    target_arch TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_connector_packages_source ON connector_packages(source);
CREATE INDEX IF NOT EXISTS idx_connector_packages_distribution ON connector_packages(distribution);
CREATE INDEX IF NOT EXISTS idx_connector_packages_install_state ON connector_packages(install_state);

-- server_admins: track which plugin instance manages each admin row.
-- The column is new and NULL by default so the FK validation pass is
-- effectively zero-cost; we still use NOT VALID + VALIDATE for safety on
-- larger deployments. The two indexes are likewise on an all-NULL column
-- and complete instantly.
ALTER TABLE server_admins
    ADD COLUMN managed_by_plugin_instance_id UUID;

ALTER TABLE server_admins
    ADD CONSTRAINT fk_server_admins_managed_by_plugin_instance_id_plugin_instances_id
    FOREIGN KEY (managed_by_plugin_instance_id) REFERENCES plugin_instances(id) ON DELETE CASCADE
    NOT VALID;

ALTER TABLE server_admins
    VALIDATE CONSTRAINT fk_server_admins_managed_by_plugin_instance_id_plugin_instances_id;

CREATE INDEX IF NOT EXISTS idx_server_admins_managed_by_plugin_instance_id
    ON server_admins(managed_by_plugin_instance_id);

CREATE INDEX IF NOT EXISTS idx_server_admins_server_id_managed_by_plugin_instance_id
    ON server_admins(server_id, managed_by_plugin_instance_id);

-- Generalized backfill: link plugin-created admin rows to their owning
-- plugin instance using the canonical 'Plugin: <Name> - %' notes pattern.
-- We pick the most recent plugin instance per (server_id, plugin_id) so a
-- single matching plugin instance per server is unambiguous.
WITH candidate_plugin_instances AS (
    SELECT DISTINCT ON (server_id, plugin_id)
        id,
        server_id,
        plugin_id
    FROM plugin_instances
    ORDER BY server_id, plugin_id, updated_at DESC, created_at DESC, id
),
matched AS (
    UPDATE server_admins sa
    SET managed_by_plugin_instance_id = cpi.id
    FROM candidate_plugin_instances cpi
    WHERE sa.managed_by_plugin_instance_id IS NULL
      AND sa.server_id = cpi.server_id
      AND sa.notes LIKE 'Plugin: % - %'
      AND (
          (cpi.plugin_id = 'server_seeder_whitelist' AND sa.notes LIKE 'Plugin: Seeder Whitelist - %')
          OR
          (cpi.plugin_id = 'squad_leader_whitelist'  AND sa.notes LIKE 'Plugin: Squad Leader Whitelist - %')
          OR
          (cpi.plugin_id NOT IN ('server_seeder_whitelist', 'squad_leader_whitelist')
              AND sa.notes ILIKE 'Plugin: ' || cpi.plugin_id || ' - %')
      )
    RETURNING sa.id
)
SELECT COUNT(*) AS matched_admin_rows FROM matched;

-- Surface plugin-template admin rows that could not be linked. Operators
-- can review these orphans and either delete them or claim them via the
-- plugin admin UI.
DO $$
DECLARE
    orphan_count BIGINT;
BEGIN
    SELECT COUNT(*) INTO orphan_count
    FROM server_admins
    WHERE managed_by_plugin_instance_id IS NULL
      AND notes LIKE 'Plugin: % - %';
    IF orphan_count > 0 THEN
        RAISE NOTICE 'server_admins backfill: % rows match the Plugin: <Name> - %% notes pattern but could not be linked to a plugin instance. Review and reconcile via the plugin admin UI.', orphan_count, '%';
    END IF;
END $$;
