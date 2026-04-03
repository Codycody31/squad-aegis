ALTER TABLE plugin_packages
    RENAME COLUMN abi_minor TO min_host_api_version;

ALTER TABLE plugin_packages
    ALTER COLUMN min_host_api_version DROP DEFAULT;

ALTER TABLE plugin_packages
    ALTER COLUMN min_host_api_version TYPE INTEGER
    USING CASE
        WHEN trim(min_host_api_version) ~ '^[0-9]+$' THEN trim(min_host_api_version)::integer
        ELSE 0
    END;

ALTER TABLE plugin_packages
    ALTER COLUMN min_host_api_version SET DEFAULT 0;

ALTER TABLE plugin_packages
    ADD COLUMN required_capabilities JSONB NOT NULL DEFAULT '[]'::jsonb;
