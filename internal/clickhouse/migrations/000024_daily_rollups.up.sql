CREATE TABLE IF NOT EXISTS squad_aegis.server_event_daily_counts (
    day Date,
    server_id UUID,
    event_type LowCardinality(String),
    event_count UInt64
)
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(day)
ORDER BY (server_id, day, event_type)
SETTINGS index_granularity = 8192;

--migration:split

CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_daily_chat_count
TO squad_aegis.server_event_daily_counts AS
SELECT toDate(sent_at) AS day, server_id, 'chat' AS event_type, count() AS event_count
FROM squad_aegis.server_player_chat_messages
GROUP BY day, server_id;

--migration:split

CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_daily_connected_count
TO squad_aegis.server_event_daily_counts AS
SELECT toDate(event_time) AS day, server_id, 'connected' AS event_type, count() AS event_count
FROM squad_aegis.server_player_connected_events
GROUP BY day, server_id;

--migration:split

CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_daily_disconnected_count
TO squad_aegis.server_event_daily_counts AS
SELECT toDate(event_time) AS day, server_id, 'disconnected' AS event_type, count() AS event_count
FROM squad_aegis.server_player_disconnected_events
GROUP BY day, server_id;

--migration:split

CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_daily_wounded_count
TO squad_aegis.server_event_daily_counts AS
SELECT toDate(event_time) AS day, server_id, 'wounded' AS event_type, count() AS event_count
FROM squad_aegis.server_player_wounded_events
GROUP BY day, server_id;

--migration:split

CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_daily_revived_count
TO squad_aegis.server_event_daily_counts AS
SELECT toDate(event_time) AS day, server_id, 'revived' AS event_type, count() AS event_count
FROM squad_aegis.server_player_revived_events
GROUP BY day, server_id;

--migration:split

CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_daily_possess_count
TO squad_aegis.server_event_daily_counts AS
SELECT toDate(event_time) AS day, server_id, 'possess' AS event_type, count() AS event_count
FROM squad_aegis.server_player_possess_events
GROUP BY day, server_id;

--migration:split

CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_daily_died_count
TO squad_aegis.server_event_daily_counts AS
SELECT toDate(event_time) AS day, server_id, 'died' AS event_type, count() AS event_count
FROM squad_aegis.server_player_died_events
GROUP BY day, server_id;

--migration:split

CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_daily_damaged_count
TO squad_aegis.server_event_daily_counts AS
SELECT toDate(event_time) AS day, server_id, 'damaged' AS event_type, count() AS event_count
FROM squad_aegis.server_player_damaged_events
GROUP BY day, server_id;

--migration:split

CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_daily_deployable_damaged_count
TO squad_aegis.server_event_daily_counts AS
SELECT toDate(event_time) AS day, server_id, 'deployable_damaged' AS event_type, count() AS event_count
FROM squad_aegis.server_deployable_damaged_events
GROUP BY day, server_id;

--migration:split

CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_daily_admin_broadcast_count
TO squad_aegis.server_event_daily_counts AS
SELECT toDate(event_time) AS day, server_id, 'admin_broadcast' AS event_type, count() AS event_count
FROM squad_aegis.server_admin_broadcast_events
GROUP BY day, server_id;

--migration:split

CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_daily_round_ended_count
TO squad_aegis.server_event_daily_counts AS
SELECT toDate(event_time) AS day, server_id, 'round_ended' AS event_type, count() AS event_count
FROM squad_aegis.server_game_events_unified
WHERE event_type = 'ROUND_ENDED'
GROUP BY day, server_id;

--migration:split

CREATE TABLE IF NOT EXISTS squad_aegis.server_info_daily (
    day Date,
    server_id UUID,
    player_count_state AggregateFunction(avg, UInt16),
    public_queue_state AggregateFunction(avg, UInt16),
    reserved_queue_state AggregateFunction(avg, UInt16),
    peak_player_count SimpleAggregateFunction(max, UInt16),
    last_activity SimpleAggregateFunction(max, DateTime64(3, 'UTC'))
)
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(day)
ORDER BY (server_id, day)
SETTINGS index_granularity = 8192;

--migration:split

CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_server_info_daily
TO squad_aegis.server_info_daily AS
SELECT
    toDate(event_time) AS day,
    server_id,
    avgState(player_count) AS player_count_state,
    avgState(public_queue) AS public_queue_state,
    avgState(reserved_queue) AS reserved_queue_state,
    max(player_count) AS peak_player_count,
    max(event_time) AS last_activity
FROM squad_aegis.server_info_metrics
GROUP BY day, server_id;

--migration:split

CREATE TABLE IF NOT EXISTS squad_aegis.server_player_daily_uniques (
    day Date,
    server_id UUID,
    players_state AggregateFunction(uniqExact, String)
)
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(day)
ORDER BY (server_id, day)
SETTINGS index_granularity = 8192;

--migration:split

CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_server_player_daily_uniques
TO squad_aegis.server_player_daily_uniques AS
SELECT
    toDate(event_time) AS day,
    server_id,
    uniqExactState(
        multiIf(
            ifNull(steam, '') != '', concat('steam:', assumeNotNull(steam)),
            ifNull(eos, '') != '', concat('eos:', assumeNotNull(eos)),
            concat('epic:', assumeNotNull(epic))
        )
    ) AS players_state
FROM squad_aegis.server_join_succeeded_events
WHERE ifNull(steam, '') != ''
   OR ifNull(eos, '') != ''
   OR ifNull(epic, '') != ''
GROUP BY day, server_id;

--migration:split

CREATE TABLE IF NOT EXISTS squad_aegis.server_workflow_daily (
    day Date,
    server_id UUID,
    workflow_id UUID,
    executions_state AggregateFunction(uniqExact, UUID)
)
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(day)
ORDER BY (server_id, workflow_id, day)
SETTINGS index_granularity = 8192;

--migration:split

CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_server_workflow_daily
TO squad_aegis.server_workflow_daily AS
SELECT
    toDate(event_time) AS day,
    server_id,
    workflow_id,
    uniqExactState(execution_id) AS executions_state
FROM squad_aegis.workflow_execution_logs
GROUP BY day, server_id, workflow_id;

--migration:split

CREATE TABLE IF NOT EXISTS squad_aegis.player_ip_activity (
    ip String,
    identifier_type LowCardinality(String),
    identifier_value String,
    server_id UUID,
    sessions SimpleAggregateFunction(sum, UInt64),
    first_seen SimpleAggregateFunction(min, DateTime64(3, 'UTC')),
    last_seen SimpleAggregateFunction(max, DateTime64(3, 'UTC'))
)
ENGINE = AggregatingMergeTree
PARTITION BY cityHash64(ip) % 32
ORDER BY (ip, identifier_type, identifier_value, server_id)
SETTINGS index_granularity = 8192;

--migration:split

CREATE MATERIALIZED VIEW IF NOT EXISTS squad_aegis.mv_player_ip_activity
TO squad_aegis.player_ip_activity AS
SELECT
    ip,
    multiIf(
        ifNull(steam, '') != '', 'steam',
        ifNull(eos, '') != '', 'eos',
        'epic'
    ) AS identifier_type,
    multiIf(
        ifNull(steam, '') != '', assumeNotNull(steam),
        ifNull(eos, '') != '', assumeNotNull(eos),
        assumeNotNull(epic)
    ) AS identifier_value,
    server_id,
    count() AS sessions,
    min(event_time) AS first_seen,
    max(event_time) AS last_seen
FROM squad_aegis.server_player_connected_events
WHERE ip != ''
  AND (
      ifNull(steam, '') != ''
      OR ifNull(eos, '') != ''
      OR ifNull(epic, '') != ''
  )
GROUP BY ip, identifier_type, identifier_value, server_id;

--migration:split

CREATE TABLE IF NOT EXISTS squad_aegis.sql_rollup_state (
    rollup_name LowCardinality(String),
    created_at DateTime64(3, 'UTC'),
    first_fully_covered_day Nullable(Date),
    backfill_complete UInt8,
    updated_at DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY rollup_name
SETTINGS index_granularity = 8192;

--migration:split

INSERT INTO squad_aegis.sql_rollup_state
    (rollup_name, created_at, first_fully_covered_day, backfill_complete, updated_at)
SELECT
    rollup_name,
    now64(3),
    if(rollup_name = 'player_ip_activity', CAST(NULL, 'Nullable(Date)'), toNullable(today() + 1)),
    0,
    now64(3)
FROM
(
    SELECT arrayJoin([
        'server_event_daily_counts',
        'server_info_daily',
        'server_player_daily_uniques',
        'server_workflow_daily',
        'player_ip_activity'
    ]) AS rollup_name
);
