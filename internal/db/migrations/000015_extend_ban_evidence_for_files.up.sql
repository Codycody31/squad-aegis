-- Extend ban_evidence table to support file uploads and text evidence
-- Make ClickHouse-specific fields nullable for non-ClickHouse evidence types
ALTER TABLE ban_evidence 
    ALTER COLUMN clickhouse_table DROP NOT NULL,
    ALTER COLUMN record_id DROP NOT NULL,
    ALTER COLUMN record_id TYPE VARCHAR(255), -- Change from UUID to VARCHAR to support chain_id strings
    ALTER COLUMN event_time DROP NOT NULL; -- Allow NULL for file uploads without timestamps

-- Add file_path column for file uploads
ALTER TABLE ban_evidence ADD COLUMN file_path VARCHAR(500);

-- Add file_name column for file uploads
ALTER TABLE ban_evidence ADD COLUMN file_name VARCHAR(255);

-- Add file_size column for file uploads (in bytes)
ALTER TABLE ban_evidence ADD COLUMN file_size BIGINT;

-- Add file_type column (MIME type)
ALTER TABLE ban_evidence ADD COLUMN file_type VARCHAR(100);

-- Add text_content column for pasted text evidence
ALTER TABLE ban_evidence ADD COLUMN text_content TEXT;

-- Update evidence_type constraint comment (no actual constraint, just documentation)
-- Evidence types now include: 'player_died', 'player_wounded', 'player_damaged', 
-- 'chat_message', 'player_connected', 'file_upload', 'text_paste'

-- Create index for file_path lookups
CREATE INDEX idx_ban_evidence_file_path ON ban_evidence(file_path) WHERE file_path IS NOT NULL;

