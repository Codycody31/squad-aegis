ALTER TABLE servers DROP CONSTRAINT IF EXISTS chk_ban_enforcement_mode;
ALTER TABLE servers DROP COLUMN IF EXISTS ban_enforcement_mode;
