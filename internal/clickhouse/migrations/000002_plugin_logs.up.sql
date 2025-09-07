--migration:split
CREATE TABLE IF NOT EXISTS squad_aegis.plugin_logs (
    log_id          UUID DEFAULT generateUUIDv4(),
    timestamp       DateTime64(3, 'UTC'),
    server_id       UUID,
    plugin_instance_id UUID,
    level           LowCardinality(String),         -- 'info', 'warn', 'error', 'debug'
    message         String CODEC(ZSTD(5)),
    error_message   Nullable(String) CODEC(ZSTD(5)), -- for error logs
    fields          String CODEC(ZSTD(5)),          -- JSON encoded fields map
    ingested_at     DateTime DEFAULT now(),
    -- Indexes for efficient querying
    INDEX idx_message_tokenbf message TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 64
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(timestamp)
ORDER BY (server_id, plugin_instance_id, timestamp, log_id)
SETTINGS index_granularity = 8192;
