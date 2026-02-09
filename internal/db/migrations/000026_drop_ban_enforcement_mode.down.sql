ALTER TABLE servers ADD COLUMN ban_enforcement_mode VARCHAR(10) NOT NULL DEFAULT 'server';
ALTER TABLE servers ADD CONSTRAINT chk_ban_enforcement_mode CHECK (ban_enforcement_mode IN ('server', 'aegis'));
