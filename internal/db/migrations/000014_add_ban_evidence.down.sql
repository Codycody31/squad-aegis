-- Drop ban_evidence table
DROP TABLE IF EXISTS ban_evidence;

-- Remove evidence_text column from server_bans
ALTER TABLE server_bans DROP COLUMN IF EXISTS evidence_text;

