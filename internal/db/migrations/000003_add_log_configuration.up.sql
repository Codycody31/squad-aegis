ALTER TABLE servers ADD COLUMN log_source_type VARCHAR(10) CHECK (log_source_type IN ('local', 'sftp', 'ftp'));
ALTER TABLE servers ADD COLUMN log_file_path TEXT;
ALTER TABLE servers ADD COLUMN log_host VARCHAR(255);
ALTER TABLE servers ADD COLUMN log_port INTEGER CHECK (log_port > 0 AND log_port <= 65535);
ALTER TABLE servers ADD COLUMN log_username VARCHAR(255);
ALTER TABLE servers ADD COLUMN log_password TEXT;
ALTER TABLE servers ADD COLUMN log_poll_frequency INTEGER DEFAULT 5 CHECK (log_poll_frequency > 0);
ALTER TABLE servers ADD COLUMN log_read_from_start BOOLEAN DEFAULT FALSE;

CREATE INDEX idx_servers_log_configured ON servers (id) WHERE log_source_type IS NOT NULL AND log_file_path IS NOT NULL;