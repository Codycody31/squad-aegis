CREATE TABLE IF NOT EXISTS squad_aegis.workflow_log_messages (
    execution_id UUID,
    workflow_id UUID,
    server_id UUID,
    step_id String,
    step_name String,
    log_time DateTime64(3, 'UTC'),
    log_level LowCardinality(String),
    message String CODEC(ZSTD(5)),
    variables String CODEC(ZSTD(5)),
    metadata String CODEC(ZSTD(5)),
    ingested_at DateTime DEFAULT now(),
    INDEX idx_log_level_tokenbf log_level TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 64,
    INDEX idx_message_tokenbf message TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 64,
    INDEX idx_step_name_tokenbf step_name TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 64
) ENGINE = MergeTree PARTITION BY toYYYYMM(log_time)
ORDER BY (server_id, workflow_id, execution_id, log_time) SETTINGS index_granularity = 8192;