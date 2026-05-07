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
    -- JSONB is normalized on storage; bytes are not byte-equal to the
    -- bundle's manifest.json. Signature verification reads signed_manifest_json.
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
-- install_state is not indexed: low-cardinality, no WHERE filters use it.

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

-- managed_by_plugin_instance_id ties an admin row to a specific plugin
-- instance. NOT VALID + VALIDATE is cheap on the new all-NULL column.
ALTER TABLE server_admins
    ADD COLUMN managed_by_plugin_instance_id UUID;

-- managed_by_plugin_id is the canonical plugin identifier (stable across
-- instance recreation). Combined with ON DELETE SET NULL on the FK below,
-- this lets a recreated plugin instance auto-claim orphan rows via
-- AddTemporaryAdmin without the data-loss risk of ON DELETE CASCADE.
ALTER TABLE server_admins
    ADD COLUMN managed_by_plugin_id TEXT;

-- SET NULL, not CASCADE: backfill could mislabel rows, and operators may
-- delete an instance without intending to drop its admin grants.
ALTER TABLE server_admins
    ADD CONSTRAINT fk_server_admins_managed_by_plugin_instance_id_plugin_instances_id
    FOREIGN KEY (managed_by_plugin_instance_id) REFERENCES plugin_instances(id) ON DELETE SET NULL
    NOT VALID;

ALTER TABLE server_admins
    VALIDATE CONSTRAINT fk_server_admins_managed_by_plugin_instance_id_plugin_instances_id;

CREATE INDEX IF NOT EXISTS idx_server_admins_managed_by_plugin_instance_id
    ON server_admins(managed_by_plugin_instance_id);

CREATE INDEX IF NOT EXISTS idx_server_admins_server_id_managed_by_plugin_instance_id
    ON server_admins(server_id, managed_by_plugin_instance_id);

-- Partial index for the orphan auto-claim lookup in AddTemporaryAdmin.
CREATE INDEX IF NOT EXISTS idx_server_admins_orphan_plugin_id
    ON server_admins(server_id, managed_by_plugin_id)
    WHERE managed_by_plugin_instance_id IS NULL AND managed_by_plugin_id IS NOT NULL;

-- Backfill links plugin-created admin rows for the two in-tree plugins whose
-- notes format is known. We restrict to single-instance (server_id, plugin_id)
-- pairs so we never arbitrarily bind a row to one of several candidate instances.
WITH plugin_instance_counts AS (
    SELECT server_id, plugin_id, COUNT(*) AS instance_count
    FROM plugin_instances
    GROUP BY server_id, plugin_id
),
candidate_plugin_instances AS (
    SELECT pi.id, pi.server_id, pi.plugin_id
    FROM plugin_instances pi
    JOIN plugin_instance_counts c
      ON c.server_id = pi.server_id
     AND c.plugin_id = pi.plugin_id
    WHERE c.instance_count = 1
),
matched AS (
    UPDATE server_admins sa
    SET managed_by_plugin_instance_id = cpi.id,
        managed_by_plugin_id = cpi.plugin_id
    FROM candidate_plugin_instances cpi
    WHERE sa.managed_by_plugin_instance_id IS NULL
      AND sa.server_id = cpi.server_id
      AND (
          (cpi.plugin_id = 'server_seeder_whitelist' AND sa.notes LIKE 'Plugin: Seeder Whitelist - %')
          OR
          (cpi.plugin_id = 'squad_leader_whitelist'  AND sa.notes LIKE 'Plugin: Squad Leader Whitelist - %')
      )
    RETURNING sa.id
)
SELECT COUNT(*) AS matched_admin_rows FROM matched;

-- Surface unlinked plugin-template admin rows so operators can review them.
DO $$
DECLARE
    orphan_count BIGINT;
BEGIN
    SELECT COUNT(*) INTO orphan_count
    FROM server_admins
    WHERE managed_by_plugin_instance_id IS NULL
      AND notes LIKE 'Plugin: % - %';
    IF orphan_count > 0 THEN
        -- %% is a literal %; only % consumes positional args.
        RAISE NOTICE 'server_admins backfill: % rows match the Plugin: <Name> - %% notes pattern but could not be linked to a plugin instance. Review and reconcile via the plugin admin UI.', orphan_count;
    END IF;
END $$;
