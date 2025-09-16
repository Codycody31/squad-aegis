CREATE TABLE IF NOT EXISTS squad_aegis.workflow_execution_logs (
    execution_id UUID,
    workflow_id UUID,
    server_id UUID,
    event_time DateTime64(3, 'UTC'),
    trigger_event_type LowCardinality(String),
    trigger_event_data String CODEC(ZSTD(5)),
    status LowCardinality(String),
    step_name String,
    step_type LowCardinality(String),
    step_order UInt32,
    step_status LowCardinality(String),
    step_input String CODEC(ZSTD(5)),
    step_output String CODEC(ZSTD(5)),
    step_error Nullable(String) CODEC(ZSTD(5)),
    step_duration_ms UInt32,
    variables String CODEC(ZSTD(5)),
    metadata String CODEC(ZSTD(5)),
    ingested_at DateTime DEFAULT now(),
    INDEX idx_trigger_event_type trigger_event_type TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 64,
    INDEX idx_step_name_tokenbf step_name TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 64
) ENGINE = MergeTree PARTITION BY toYYYYMM(event_time)
ORDER BY (
        server_id,
        workflow_id,
        execution_id,
        step_order,
        event_time
    ) SETTINGS index_granularity = 8192;
--migration:split
CREATE TABLE IF NOT EXISTS squad_aegis.workflow_execution_summary (
    execution_id UUID,
    workflow_id UUID,
    server_id UUID,
    workflow_name String,
    started_at DateTime64(3, 'UTC'),
    completed_at Nullable(DateTime64(3, 'UTC')),
    status LowCardinality(String),
    trigger_event_type LowCardinality(String),
    total_steps UInt32,
    completed_steps UInt32,
    failed_steps UInt32,
    skipped_steps UInt32,
    total_duration_ms UInt32,
    error_message Nullable(String) CODEC(ZSTD(5)),
    ingested_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(ingested_at) PARTITION BY toYYYYMM(started_at)
ORDER BY (server_id, workflow_id, execution_id, started_at) SETTINGS index_granularity = 8192;