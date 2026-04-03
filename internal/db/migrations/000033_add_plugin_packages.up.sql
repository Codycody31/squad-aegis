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
    signature_verified BOOLEAN NOT NULL DEFAULT false,
    unsafe BOOLEAN NOT NULL DEFAULT false,
    checksum TEXT NOT NULL DEFAULT '',
    abi_minor TEXT NOT NULL DEFAULT '',
    target_os TEXT NOT NULL DEFAULT '',
    target_arch TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_plugin_packages_source ON plugin_packages(source);
CREATE INDEX IF NOT EXISTS idx_plugin_packages_distribution ON plugin_packages(distribution);
CREATE INDEX IF NOT EXISTS idx_plugin_packages_install_state ON plugin_packages(install_state);

