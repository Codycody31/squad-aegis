ALTER TABLE plugin_packages
    DROP COLUMN IF EXISTS required_capabilities;

ALTER TABLE plugin_packages
    ALTER COLUMN min_host_api_version DROP DEFAULT;

ALTER TABLE plugin_packages
    ALTER COLUMN min_host_api_version TYPE TEXT
    USING min_host_api_version::text;

ALTER TABLE plugin_packages
    ALTER COLUMN min_host_api_version SET DEFAULT '';

ALTER TABLE plugin_packages
    RENAME COLUMN min_host_api_version TO abi_minor;
