ALTER TABLE connector_packages
    DROP COLUMN IF EXISTS signature_expires_at,
    DROP COLUMN IF EXISTS signature_signed_at,
    DROP COLUMN IF EXISTS signature_key_id,
    DROP COLUMN IF EXISTS signed_manifest_json;

ALTER TABLE plugin_packages
    DROP COLUMN IF EXISTS signature_expires_at,
    DROP COLUMN IF EXISTS signature_signed_at,
    DROP COLUMN IF EXISTS signature_key_id,
    DROP COLUMN IF EXISTS signed_manifest_json;

ALTER TABLE plugin_packages ADD COLUMN checksum TEXT NOT NULL DEFAULT '';
ALTER TABLE connector_packages ADD COLUMN checksum TEXT NOT NULL DEFAULT '';
