CREATE TABLE IF NOT EXISTS squad_aegis.player_identity_snapshots (
    snapshot_id UUID,
    canonical_id String,
    primary_steam_id String,
    primary_eos_id String,
    primary_epic_id String,
    primary_name String,
    all_steam_ids Array(String),
    all_eos_ids Array(String),
    all_epic_ids Array(String),
    all_names Array(String),
    total_sessions UInt64,
    first_seen DateTime64(3, 'UTC'),
    last_seen DateTime64(3, 'UTC'),
    computed_at DateTime64(3, 'UTC') DEFAULT now64(3)
)
ENGINE = MergeTree
PARTITION BY snapshot_id
ORDER BY (snapshot_id, canonical_id)
SETTINGS index_granularity = 8192;

--migration:split

CREATE TABLE IF NOT EXISTS squad_aegis.player_identity_lookup_snapshots (
    snapshot_id UUID,
    identifier_type LowCardinality(String),
    identifier_value String,
    canonical_id String,
    computed_at DateTime64(3, 'UTC') DEFAULT now64(3),
    INDEX idx_identity_value_ngram lowerUTF8(identifier_value)
        TYPE ngrambf_v1(3, 10000, 3, 0) GRANULARITY 1
)
ENGINE = MergeTree
PARTITION BY snapshot_id
ORDER BY (snapshot_id, identifier_type, identifier_value, canonical_id)
SETTINGS index_granularity = 8192;

--migration:split

CREATE TABLE IF NOT EXISTS squad_aegis.player_identity_current (
    pointer_key UInt8 DEFAULT 1,
    snapshot_id UUID,
    identity_count UInt64,
    lookup_count UInt64,
    published_at DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree(published_at)
ORDER BY snapshot_id
SETTINGS index_granularity = 8192;

--migration:split

INSERT INTO squad_aegis.player_identity_snapshots (
    snapshot_id,
    canonical_id,
    primary_steam_id,
    primary_eos_id,
    primary_epic_id,
    primary_name,
    all_steam_ids,
    all_eos_ids,
    all_epic_ids,
    all_names,
    total_sessions,
    first_seen,
    last_seen,
    computed_at
)
SELECT
    toUUID('00000000-0000-0000-0000-000000000001'),
    canonical_id,
    primary_steam_id,
    primary_eos_id,
    primary_epic_id,
    primary_name,
    all_steam_ids,
    all_eos_ids,
    all_epic_ids,
    all_names,
    total_sessions,
    first_seen,
    last_seen,
    toDateTime64(computed_at, 3, 'UTC')
FROM squad_aegis.player_identities;

--migration:split

INSERT INTO squad_aegis.player_identity_lookup_snapshots (
    snapshot_id,
    identifier_type,
    identifier_value,
    canonical_id,
    computed_at
)
SELECT
    toUUID('00000000-0000-0000-0000-000000000001'),
    identifier_type,
    identifier_value,
    canonical_id,
    toDateTime64(computed_at, 3, 'UTC')
FROM squad_aegis.player_identity_lookup;

--migration:split

INSERT INTO squad_aegis.player_identity_current (
    pointer_key,
    snapshot_id,
    identity_count,
    lookup_count,
    published_at
)
SELECT
    1,
    toUUID('00000000-0000-0000-0000-000000000001'),
    snapshot_identity_count,
    snapshot_lookup_count,
    now64(3)
FROM
(
    SELECT
        (SELECT count() FROM squad_aegis.player_identities) AS source_identity_count,
        (SELECT count() FROM squad_aegis.player_identity_lookup) AS source_lookup_count,
        (
            SELECT count()
            FROM squad_aegis.player_identity_snapshots
            WHERE snapshot_id = toUUID('00000000-0000-0000-0000-000000000001')
        ) AS snapshot_identity_count,
        (
            SELECT count()
            FROM squad_aegis.player_identity_lookup_snapshots
            WHERE snapshot_id = toUUID('00000000-0000-0000-0000-000000000001')
        ) AS snapshot_lookup_count
)
WHERE source_identity_count = snapshot_identity_count
  AND source_lookup_count = snapshot_lookup_count;
