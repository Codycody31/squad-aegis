-- Player identities table for consolidated player records
-- This table stores pre-computed transitive identity resolution
-- linking all Steam IDs and EOS IDs that belong to the same player

CREATE TABLE IF NOT EXISTS squad_aegis.player_identities (
    -- Canonical identity (unique identifier for this player)
    canonical_id String,

    -- Primary identifiers (most recently used or most common)
    primary_steam_id String,
    primary_eos_id String,
    primary_name String,

    -- All linked identifiers
    all_steam_ids Array(String),
    all_eos_ids Array(String),
    all_names Array(String),

    -- Aggregated statistics
    total_sessions UInt64,
    first_seen DateTime64(3, 'UTC'),
    last_seen DateTime64(3, 'UTC'),

    -- Metadata
    computed_at DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(computed_at)
ORDER BY canonical_id
SETTINGS index_granularity = 8192;

--migration:split

-- Lookup table for fast searches by any identifier
-- Allows O(1) lookup from any steam_id, eos_id, or name to canonical_id

CREATE TABLE IF NOT EXISTS squad_aegis.player_identity_lookup (
    identifier_type LowCardinality(String),  -- 'steam', 'eos', 'name'
    identifier_value String,
    canonical_id String,
    computed_at DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(computed_at)
ORDER BY (identifier_type, identifier_value)
SETTINGS index_granularity = 8192;
