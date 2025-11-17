-- Add is_evidence flag to player death events to prevent deletion
ALTER TABLE squad_aegis.server_player_died_events 
ADD COLUMN IF NOT EXISTS is_evidence UInt8 DEFAULT 0;

-- Add is_evidence flag to player wounded events
ALTER TABLE squad_aegis.server_player_wounded_events 
ADD COLUMN IF NOT EXISTS is_evidence UInt8 DEFAULT 0;

-- Add is_evidence flag to player damaged events
ALTER TABLE squad_aegis.server_player_damaged_events 
ADD COLUMN IF NOT EXISTS is_evidence UInt8 DEFAULT 0;

-- Add is_evidence flag to chat messages
ALTER TABLE squad_aegis.server_player_chat_messages 
ADD COLUMN IF NOT EXISTS is_evidence UInt8 DEFAULT 0;

-- Add is_evidence flag to player connected events
ALTER TABLE squad_aegis.server_player_connected_events 
ADD COLUMN IF NOT EXISTS is_evidence UInt8 DEFAULT 0;

--migration:split
-- Create a table to track evidence links (for reference and cleanup)
CREATE TABLE IF NOT EXISTS squad_aegis.ban_evidence_links (
    link_id UUID DEFAULT generateUUIDv4(),
    ban_id UUID,
    evidence_type LowCardinality(String),
    clickhouse_table LowCardinality(String),
    record_id UUID,
    server_id UUID,
    event_time DateTime64(3, 'UTC'),
    metadata String,
    created_at DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (ban_id, server_id, event_time, link_id);

--migration:split
-- Create index for efficient lookups
CREATE INDEX IF NOT EXISTS idx_ban_evidence_record ON squad_aegis.ban_evidence_links (clickhouse_table, record_id) TYPE bloom_filter GRANULARITY 64;

