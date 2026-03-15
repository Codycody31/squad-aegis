ALTER TABLE server_bans ADD COLUMN duration INTEGER NOT NULL DEFAULT 0;

UPDATE server_bans
SET duration = GREATEST(CEIL(EXTRACT(EPOCH FROM (expires_at - created_at)) / 86400), 1)
WHERE expires_at IS NOT NULL;

ALTER TABLE server_bans DROP COLUMN expires_at;
