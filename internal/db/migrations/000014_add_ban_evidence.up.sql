-- Add evidence_text column to server_bans table
ALTER TABLE server_bans ADD COLUMN evidence_text TEXT;

-- Create ban_evidence table to link bans to ClickHouse records
CREATE TABLE ban_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ban_id UUID NOT NULL REFERENCES server_bans(id) ON DELETE CASCADE,
    evidence_type VARCHAR(50) NOT NULL, -- 'player_died', 'player_wounded', 'player_damaged', 'chat_message', 'player_connected', etc.
    clickhouse_table VARCHAR(100) NOT NULL, -- The ClickHouse table name
    record_id UUID NOT NULL, -- The event_id or message_id from ClickHouse
    server_id UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    event_time TIMESTAMPTZ NOT NULL, -- Timestamp of the event for display purposes
    metadata JSONB, -- Additional metadata about the evidence (e.g., victim name, weapon, damage, message content)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes for efficient lookups
CREATE INDEX idx_ban_evidence_ban_id ON ban_evidence(ban_id);
CREATE INDEX idx_ban_evidence_server_id ON ban_evidence(server_id);
CREATE INDEX idx_ban_evidence_record_id ON ban_evidence(record_id);
CREATE INDEX idx_ban_evidence_event_time ON ban_evidence(event_time DESC);

-- Create a composite index for checking if evidence exists
CREATE INDEX idx_ban_evidence_record_lookup ON ban_evidence(clickhouse_table, record_id);

