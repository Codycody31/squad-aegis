ALTER TABLE servers ADD COLUMN squad_game_path TEXT;

UPDATE servers
SET squad_game_path = CASE
	WHEN log_file_path IS NOT NULL AND log_file_path LIKE '%/Saved/Logs/%'
		THEN regexp_replace(log_file_path, '/Saved/Logs/[^/]+$', '')
	WHEN log_file_path IS NOT NULL AND log_file_path LIKE '%\Saved\Logs\%'
		THEN regexp_replace(log_file_path, '\\Saved\\Logs\\[^\\]+$', '')
	WHEN bans_cfg_path IS NOT NULL AND bans_cfg_path LIKE '%/ServerConfig/%'
		THEN regexp_replace(bans_cfg_path, '/ServerConfig/[^/]+$', '')
	WHEN bans_cfg_path IS NOT NULL AND bans_cfg_path LIKE '%\ServerConfig\%'
		THEN regexp_replace(bans_cfg_path, '\\ServerConfig\\[^\\]+$', '')
	ELSE squad_game_path
END
WHERE squad_game_path IS NULL;

DROP INDEX IF EXISTS idx_servers_log_configured;

ALTER TABLE servers DROP COLUMN IF EXISTS log_file_path;
ALTER TABLE servers DROP COLUMN IF EXISTS bans_cfg_path;

CREATE INDEX idx_servers_log_configured ON servers (id) WHERE log_source_type IS NOT NULL AND squad_game_path IS NOT NULL;

-- Backfill squad_game_path from server_motd_config.motd_file_path for servers
-- that still have NULL squad_game_path (e.g. servers using custom MOTD upload
-- credentials where log_file_path/bans_cfg_path were not set).
UPDATE servers s
SET squad_game_path = CASE
	WHEN mc.motd_file_path LIKE '%/ServerConfig/%'
		THEN regexp_replace(mc.motd_file_path, '/ServerConfig/[^/]+$', '')
	WHEN mc.motd_file_path LIKE '%\ServerConfig\%'
		THEN regexp_replace(mc.motd_file_path, '\\ServerConfig\\[^\\]+$', '')
	ELSE s.squad_game_path
END
FROM server_motd_config mc
WHERE mc.server_id = s.id
	AND s.squad_game_path IS NULL
	AND mc.motd_file_path IS NOT NULL;

ALTER TABLE server_motd_config DROP COLUMN IF EXISTS motd_file_path;
