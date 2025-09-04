DROP INDEX IF EXISTS idx_servers_log_configured;

ALTER TABLE servers DROP COLUMN IF EXISTS log_read_from_start;
ALTER TABLE servers DROP COLUMN IF EXISTS log_poll_frequency;
ALTER TABLE servers DROP COLUMN IF EXISTS log_password;
ALTER TABLE servers DROP COLUMN IF EXISTS log_username;
ALTER TABLE servers DROP COLUMN IF EXISTS log_port;
ALTER TABLE servers DROP COLUMN IF EXISTS log_host;
ALTER TABLE servers DROP COLUMN IF EXISTS log_file_path;
ALTER TABLE servers DROP COLUMN IF EXISTS log_source_type;
