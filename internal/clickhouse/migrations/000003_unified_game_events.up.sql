CREATE TABLE IF NOT EXISTS squad_aegis.server_game_events_unified (
    event_id UUID DEFAULT generateUUIDv4(),
    event_time DateTime64(3, 'UTC'),
    server_id UUID,
    chain_id String,
    event_type LowCardinality(String),
    winner Nullable(String),
    layer Nullable(String),
    team Nullable(UInt8),
    subfaction Nullable(String),
    faction Nullable(String),
    action Nullable(String),
    tickets Nullable(UInt32),
    level Nullable(String),
    dlc Nullable(String),
    map_classname Nullable(String),
    layer_classname Nullable(String),
    from_state Nullable(String),
    to_state Nullable(String),
    winner_data Nullable(String),
    loser_data Nullable(String),
    metadata Nullable(String),
    raw_log String CODEC(ZSTD(5)),
    ingested_at DateTime DEFAULT now(),
    INDEX idx_event_type event_type TYPE
    set(0) GRANULARITY 64,
        INDEX idx_winner winner TYPE
    set(0) GRANULARITY 64,
        INDEX idx_layer layer TYPE
    set(0) GRANULARITY 64
) ENGINE = MergeTree PARTITION BY (toYYYYMM(event_time), event_type)
ORDER BY (server_id, event_time, event_type, event_id) SETTINGS index_granularity = 8192;
--migration:split
CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_round_ended_events TO squad_aegis.server_round_ended_events AS
SELECT event_time,
    server_id,
    chain_id,
    winner,
    layer,
    winner_data as winner_json,
    loser_data as loser_json,
    ingested_at
FROM squad_aegis.server_game_events_unified
WHERE event_type = 'ROUND_ENDED';
--migration:split
CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_new_game_events TO squad_aegis.server_new_game_events AS
SELECT event_time,
    server_id,
    chain_id,
    team,
    subfaction,
    faction,
    action,
    toString(tickets) as tickets,
    layer,
    level,
    dlc,
    map_classname,
    layer_classname,
    ingested_at
FROM squad_aegis.server_game_events_unified
WHERE event_type = 'NEW_GAME';