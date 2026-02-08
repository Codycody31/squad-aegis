ALTER TABLE servers ADD COLUMN log_file_path TEXT;
ALTER TABLE servers ADD COLUMN bans_cfg_path TEXT;

DROP INDEX IF EXISTS idx_servers_log_configured;
CREATE INDEX idx_servers_log_configured ON servers (id) WHERE log_source_type IS NOT NULL AND log_file_path IS NOT NULL;

ALTER TABLE servers DROP COLUMN IF EXISTS squad_game_path;

ALTER TABLE server_motd_config ADD COLUMN motd_file_path TEXT DEFAULT '/SquadGame/ServerConfig/MOTD.cfg';
