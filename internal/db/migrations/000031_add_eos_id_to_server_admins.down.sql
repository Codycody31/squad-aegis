ALTER TABLE server_admins DROP CONSTRAINT IF EXISTS chk_server_admin_has_subject;

CREATE TABLE IF NOT EXISTS server_admins_eos_backup AS
SELECT * FROM server_admins WHERE user_id IS NULL AND steam_id IS NULL AND eos_id IS NOT NULL;

DELETE FROM server_admins
WHERE user_id IS NULL AND steam_id IS NULL AND eos_id IS NOT NULL;

DROP INDEX IF EXISTS idx_server_admins_eos_id;
ALTER TABLE server_admins DROP COLUMN IF EXISTS eos_id;
