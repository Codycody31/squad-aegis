ALTER TABLE server_bans DROP CONSTRAINT IF EXISTS chk_ban_has_identifier;
-- Delete EOS-only bans before re-adding NOT NULL on steam_id
DELETE FROM server_bans WHERE steam_id IS NULL;
ALTER TABLE server_bans ALTER COLUMN steam_id SET NOT NULL;
DROP INDEX IF EXISTS idx_server_bans_eos_id;
ALTER TABLE server_bans DROP COLUMN IF EXISTS eos_id;
