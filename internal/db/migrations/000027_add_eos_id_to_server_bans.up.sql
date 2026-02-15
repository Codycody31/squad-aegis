-- Add optional EOS ID column for banning by EOS identifier
ALTER TABLE server_bans ADD COLUMN eos_id VARCHAR(32);
CREATE INDEX idx_server_bans_eos_id ON server_bans(eos_id);

-- Relax steam_id NOT NULL constraint (EOS-only bans won't have a Steam ID)
ALTER TABLE server_bans ALTER COLUMN steam_id DROP NOT NULL;

-- At least one identifier must be present
ALTER TABLE server_bans ADD CONSTRAINT chk_ban_has_identifier
CHECK (steam_id IS NOT NULL OR eos_id IS NOT NULL);
