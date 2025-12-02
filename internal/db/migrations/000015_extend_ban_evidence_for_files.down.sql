-- Remove file-related columns
ALTER TABLE ban_evidence DROP COLUMN IF EXISTS file_path;
ALTER TABLE ban_evidence DROP COLUMN IF EXISTS file_name;
ALTER TABLE ban_evidence DROP COLUMN IF EXISTS file_size;
ALTER TABLE ban_evidence DROP COLUMN IF EXISTS file_type;
ALTER TABLE ban_evidence DROP COLUMN IF EXISTS text_content;

-- Drop file_path index
DROP INDEX IF EXISTS idx_ban_evidence_file_path;

-- Revert nullable changes (make fields required again)
ALTER TABLE ban_evidence 
    ALTER COLUMN clickhouse_table SET NOT NULL,
    ALTER COLUMN record_id SET NOT NULL,
    ALTER COLUMN record_id TYPE UUID USING record_id::uuid,
    ALTER COLUMN event_time SET NOT NULL;

