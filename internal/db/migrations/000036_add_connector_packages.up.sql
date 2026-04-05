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
