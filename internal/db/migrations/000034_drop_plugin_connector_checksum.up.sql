ALTER TABLE plugin_packages DROP COLUMN checksum;
ALTER TABLE connector_packages DROP COLUMN checksum;

ALTER TABLE plugin_packages
    ADD COLUMN signed_manifest_json TEXT NOT NULL DEFAULT '',
    ADD COLUMN signature_key_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN signature_signed_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN signature_expires_at TIMESTAMP WITH TIME ZONE;

ALTER TABLE connector_packages
    ADD COLUMN signed_manifest_json TEXT NOT NULL DEFAULT '',
    ADD COLUMN signature_key_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN signature_signed_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN signature_expires_at TIMESTAMP WITH TIME ZONE;
