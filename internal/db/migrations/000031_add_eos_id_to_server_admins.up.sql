ALTER TABLE server_admins ADD COLUMN eos_id VARCHAR(32);
CREATE INDEX idx_server_admins_eos_id ON server_admins(eos_id);

ALTER TABLE server_admins ADD CONSTRAINT chk_server_admin_has_subject
CHECK (user_id IS NOT NULL OR steam_id IS NOT NULL OR eos_id IS NOT NULL);
