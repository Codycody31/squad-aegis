CREATE TABLE IF NOT EXISTS squad_aegis.server_player_disconnected_events (
    event_time DateTime64(3, 'UTC'),
    server_id UUID,
    chain_id String,
    player_controller String,
    player_suffix String,
    team Nullable(String),
    ip String,
    steam Nullable(String),
    eos Nullable(String),
    ingest_ts DateTime DEFAULT now()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(event_time)
ORDER BY (server_id, event_time)