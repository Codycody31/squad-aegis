ALTER TABLE plugin_packages
    ADD COLUMN IF NOT EXISTS manifest_signature TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS manifest_public_key TEXT NOT NULL DEFAULT '';

ALTER TABLE connector_packages
    ADD COLUMN IF NOT EXISTS manifest_signature TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS manifest_public_key TEXT NOT NULL DEFAULT '';
