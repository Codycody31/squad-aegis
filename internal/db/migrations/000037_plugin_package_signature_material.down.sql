ALTER TABLE plugin_packages
    DROP COLUMN IF EXISTS manifest_signature,
    DROP COLUMN IF EXISTS manifest_public_key;

ALTER TABLE connector_packages
    DROP COLUMN IF EXISTS manifest_signature,
    DROP COLUMN IF EXISTS manifest_public_key;
