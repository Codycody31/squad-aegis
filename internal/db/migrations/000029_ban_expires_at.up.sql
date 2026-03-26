-- Add expires_at column (NULL = permanent)
ALTER TABLE server_bans ADD COLUMN expires_at TIMESTAMP;

-- Populate from existing data: duration=0 stays NULL (permanent),
-- duration>0 computes the actual expiry
UPDATE server_bans
SET expires_at = created_at + (duration * INTERVAL '1 day')
WHERE duration > 0;

-- Drop the old column
ALTER TABLE server_bans DROP COLUMN duration;
