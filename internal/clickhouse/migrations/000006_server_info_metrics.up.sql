CREATE TABLE IF NOT EXISTS squad_aegis.server_info_metrics (
    event_time          DateTime64(3, 'UTC'),
    server_id           UUID,
    player_count        UInt16,
    public_queue        UInt16,
    reserved_queue      UInt16,
    ingested_at         DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (server_id, event_time)
SETTINGS index_granularity = 8192;
