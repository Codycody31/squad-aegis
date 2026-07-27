# Database performance operations

This runbook covers the PostgreSQL and ClickHouse changes introduced by the
cursor-pagination and rollup redesign. Production support starts at PostgreSQL
18 and ClickHouse 25.3.

## Deployment sequence

1. Build the PostgreSQL indexes with their individual concurrent migrations.
2. Drain every old ClickHouse writer. In a multi-instance deployment, stopping
   only one instance is not sufficient.
3. Apply the ClickHouse compatibility, snapshot, skip-index, and rollup DDL.
   Keep ingestion paused until all materialized views exist. The migration
   records the following day as the first fully covered daily-rollup day, so
   the current partial day remains a raw-data day.
4. Deploy the backend and web client together. Offset pagination is not
   backward compatible with the cursor API.
5. Resume ingestion and check the PostgreSQL and ClickHouse query logs.
6. Materialize ClickHouse skip indexes later, one index and table at a time.
7. Backfill rollups later, one source and calendar month at a time.

ClickHouse startup migrations add index definitions only. They never run
`MATERIALIZE INDEX`, a historical rollup backfill, or `OPTIMIZE ... FINAL`.

## PostgreSQL observability

`pg_stat_statements` must be present in `shared_preload_libraries` before
PostgreSQL starts. The supplied Compose configuration enables it; external
deployments must add it to their PostgreSQL configuration and restart the
server before the extension migration runs.

Check high-total-time and high-mean-time statements without exposing bind
values:

```sql
SELECT
    queryid,
    calls,
    total_exec_time,
    mean_exec_time,
    rows,
    shared_blks_hit,
    shared_blks_read,
    left(query, 240) AS query_sample
FROM pg_stat_statements
WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
ORDER BY total_exec_time DESC
LIMIT 30;
```

Reset statistics only at a deliberate measurement boundary:

```sql
SELECT pg_stat_statements_reset();
```

For a cursor query, use representative filters and `EXPLAIN (ANALYZE, BUFFERS,
WAL, FORMAT TEXT)`. Unfiltered audit pagination should use
`idx_audit_logs_cursor` or `idx_audit_logs_server_cursor` and scan no more than
twice the requested limit.

Concurrent-index migrations cannot run inside a transaction block. Each file
therefore contains one `CREATE INDEX CONCURRENTLY` or one
`DROP INDEX CONCURRENTLY` statement. If one fails, inspect
`pg_index.indisvalid`, drop only the invalid target index concurrently, and
reapply that migration.

## ClickHouse query-log analysis

Ensure `log_queries` is enabled. Flush buffered log records before an
interactive investigation:

```sql
SYSTEM FLUSH LOGS;
```

Inspect the application’s slow reads:

```sql
SELECT
    event_time,
    query_id,
    query_duration_ms,
    read_rows,
    read_bytes,
    result_rows,
    memory_usage,
    normalized_query_hash,
    left(query, 300) AS query_sample
FROM system.query_log
WHERE event_time >= now() - INTERVAL 1 HOUR
  AND type = 'QueryFinish'
  AND query_kind = 'Select'
  AND current_database = 'squad_aegis'
ORDER BY query_duration_ms DESC
LIMIT 50;
```

Failures are in the same table with `type IN ('ExceptionBeforeStart',
'ExceptionWhileProcessing')`. Query labels such as `sa.audit.list`,
`sa.player.combat`, and `sa.metrics.timeline` should be used to correlate an
endpoint with its statements.

## Skip-index materialization

New parts receive the indexes immediately. Historical parts do not. Run only
one command at a time during a maintenance window:

```sql
ALTER TABLE squad_aegis.server_player_chat_messages
    MATERIALIZE INDEX idx_chat_player_bloom
    SETTINGS max_threads = 2, max_memory_usage = 4000000000;

ALTER TABLE squad_aegis.server_player_chat_messages
    MATERIALIZE INDEX idx_chat_message_ngram
    SETTINGS max_threads = 2, max_memory_usage = 4000000000;

ALTER TABLE squad_aegis.server_player_connected_events
    MATERIALIZE INDEX idx_connected_identity_bloom
    SETTINGS max_threads = 2, max_memory_usage = 4000000000;

ALTER TABLE squad_aegis.server_player_disconnected_events
    MATERIALIZE INDEX idx_disconnected_identity_bloom
    SETTINGS max_threads = 2, max_memory_usage = 4000000000;

ALTER TABLE squad_aegis.server_join_succeeded_events
    MATERIALIZE INDEX idx_join_identity_bloom
    SETTINGS max_threads = 2, max_memory_usage = 4000000000;

ALTER TABLE squad_aegis.server_player_possess_events
    MATERIALIZE INDEX idx_possess_identity_bloom
    SETTINGS max_threads = 2, max_memory_usage = 4000000000;

ALTER TABLE squad_aegis.server_player_died_events
    MATERIALIZE INDEX idx_died_identity_bloom
    SETTINGS max_threads = 2, max_memory_usage = 4000000000;

ALTER TABLE squad_aegis.server_player_wounded_events
    MATERIALIZE INDEX idx_wounded_identity_bloom
    SETTINGS max_threads = 2, max_memory_usage = 4000000000;

ALTER TABLE squad_aegis.server_player_damaged_events
    MATERIALIZE INDEX idx_damaged_identity_bloom
    SETTINGS max_threads = 2, max_memory_usage = 4000000000;

ALTER TABLE squad_aegis.server_player_revived_events
    MATERIALIZE INDEX idx_revived_identity_bloom
    SETTINGS max_threads = 2, max_memory_usage = 4000000000;

ALTER TABLE squad_aegis.player_rule_violations
    MATERIALIZE INDEX idx_rule_violation_identity_bloom
    SETTINGS max_threads = 2, max_memory_usage = 4000000000;

ALTER TABLE squad_aegis.plugin_logs
    MATERIALIZE INDEX idx_plugin_text_ngram
    SETTINGS max_threads = 2, max_memory_usage = 4000000000;

ALTER TABLE squad_aegis.player_identity_lookup_snapshots
    MATERIALIZE INDEX idx_identity_value_ngram
    SETTINGS max_threads = 2, max_memory_usage = 4000000000;
```

Follow progress in `system.mutations`:

```sql
SELECT
    database,
    table,
    mutation_id,
    command,
    parts_to_do,
    is_done,
    latest_fail_reason
FROM system.mutations
WHERE database = 'squad_aegis'
ORDER BY create_time DESC;
```

Validate the exact production expression, not a similar predicate:

```sql
EXPLAIN indexes = 1
SELECT event_time, server_id, victim_name, attacker_name
FROM squad_aegis.server_player_died_events
WHERE server_id = toUUID('00000000-0000-0000-0000-000000000000')
  AND event_time >= now() - INTERVAL 30 DAY
  AND (
      victim_steam = '76561198000000000'
      OR attacker_steam = '76561198000000000'
  )
ORDER BY event_time DESC
LIMIT 51;
```

Promotion requires at least 80% granule pruning on a representative selective
query. Record the `Granules` before and after each materialization. Do not
materialize the next table while a mutation is still running.

After both n-gram indexes meet the pruning target, remove the old token
indexes:

```sql
ALTER TABLE squad_aegis.server_player_chat_messages
    DROP INDEX IF EXISTS idx_msg_tokenbf;

ALTER TABLE squad_aegis.plugin_logs
    DROP INDEX IF EXISTS idx_message_tokenbf;
```

Those two drops are intentionally operational rather than startup migrations:
the old indexes remain available until the replacements have been
materialized and measured.

## Hybrid rollup reads

`squad_aegis.sql_rollup_state` is the coverage contract. Daily queries read raw
tables before `first_fully_covered_day` and rollup tables on or after that day.
They must not read both sources for the same day.

Inspect boundaries with:

```sql
SELECT
    rollup_name,
    created_at,
    first_fully_covered_day,
    backfill_complete,
    updated_at
FROM squad_aegis.sql_rollup_state
ORDER BY rollup_name;
```

`player_ip_activity` is different: consumers use the raw corrected query until
`backfill_complete = 1`. Do not publish that marker after a partial IP
backfill.

## Bounded rollup backfills

The following examples process January 2026. Change both bounds together and
run one source per command. Keep the half-open interval; it prevents adjacent
months from double counting.

For each event-count source, substitute only a vetted row from this mapping:

| Event label | Source table | Time column | Optional predicate |
| --- | --- | --- | --- |
| `chat` | `server_player_chat_messages` | `sent_at` | none |
| `connected` | `server_player_connected_events` | `event_time` | none |
| `disconnected` | `server_player_disconnected_events` | `event_time` | none |
| `wounded` | `server_player_wounded_events` | `event_time` | none |
| `revived` | `server_player_revived_events` | `event_time` | none |
| `possess` | `server_player_possess_events` | `event_time` | none |
| `died` | `server_player_died_events` | `event_time` | none |
| `damaged` | `server_player_damaged_events` | `event_time` | none |
| `deployable_damaged` | `server_deployable_damaged_events` | `event_time` | none |
| `admin_broadcast` | `server_admin_broadcast_events` | `event_time` | none |
| `round_ended` | `server_game_events_unified` | `event_time` | `event_type = 'ROUND_ENDED'` |

Example for chat:

```sql
INSERT INTO squad_aegis.server_event_daily_counts
    (day, server_id, event_type, event_count)
SELECT
    toDate(sent_at),
    server_id,
    'chat',
    count()
FROM squad_aegis.server_player_chat_messages
WHERE sent_at >= toDateTime64('2026-01-01 00:00:00', 3, 'UTC')
  AND sent_at <  toDateTime64('2026-02-01 00:00:00', 3, 'UTC')
GROUP BY toDate(sent_at), server_id
SETTINGS max_threads = 2, max_memory_usage = 4000000000;
```

Server-info states:

```sql
INSERT INTO squad_aegis.server_info_daily
SELECT
    toDate(event_time),
    server_id,
    avgState(player_count),
    avgState(public_queue),
    avgState(reserved_queue),
    max(player_count),
    max(event_time)
FROM squad_aegis.server_info_metrics
WHERE event_time >= toDateTime64('2026-01-01 00:00:00', 3, 'UTC')
  AND event_time <  toDateTime64('2026-02-01 00:00:00', 3, 'UTC')
GROUP BY toDate(event_time), server_id
SETTINGS max_threads = 2, max_memory_usage = 4000000000;
```

Daily unique players:

```sql
INSERT INTO squad_aegis.server_player_daily_uniques
SELECT
    toDate(event_time),
    server_id,
    uniqExactState(
        multiIf(
            ifNull(steam, '') != '', concat('steam:', assumeNotNull(steam)),
            ifNull(eos, '') != '', concat('eos:', assumeNotNull(eos)),
            concat('epic:', assumeNotNull(epic))
        )
    )
FROM squad_aegis.server_join_succeeded_events
WHERE event_time >= toDateTime64('2026-01-01 00:00:00', 3, 'UTC')
  AND event_time <  toDateTime64('2026-02-01 00:00:00', 3, 'UTC')
  AND (
      ifNull(steam, '') != ''
      OR ifNull(eos, '') != ''
      OR ifNull(epic, '') != ''
  )
GROUP BY toDate(event_time), server_id
SETTINGS max_threads = 2, max_memory_usage = 4000000000;
```

Workflow executions:

```sql
INSERT INTO squad_aegis.server_workflow_daily
SELECT
    toDate(event_time),
    server_id,
    workflow_id,
    uniqExactState(execution_id)
FROM squad_aegis.workflow_execution_logs
WHERE event_time >= toDateTime64('2026-01-01 00:00:00', 3, 'UTC')
  AND event_time <  toDateTime64('2026-02-01 00:00:00', 3, 'UTC')
GROUP BY toDate(event_time), server_id, workflow_id
SETTINGS max_threads = 2, max_memory_usage = 4000000000;
```

IP activity:

```sql
INSERT INTO squad_aegis.player_ip_activity
SELECT
    ip,
    multiIf(
        ifNull(steam, '') != '', 'steam',
        ifNull(eos, '') != '', 'eos',
        'epic'
    ),
    multiIf(
        ifNull(steam, '') != '', assumeNotNull(steam),
        ifNull(eos, '') != '', assumeNotNull(eos),
        assumeNotNull(epic)
    ),
    server_id,
    count(),
    min(event_time),
    max(event_time)
FROM squad_aegis.server_player_connected_events
WHERE event_time >= toDateTime64('2026-01-01 00:00:00', 3, 'UTC')
  AND event_time <  toDateTime64('2026-02-01 00:00:00', 3, 'UTC')
  AND ip != ''
  AND (
      ifNull(steam, '') != ''
      OR ifNull(eos, '') != ''
      OR ifNull(epic, '') != ''
  )
GROUP BY
    ip,
    multiIf(
        ifNull(steam, '') != '', 'steam',
        ifNull(eos, '') != '', 'eos',
        'epic'
    ),
    multiIf(
        ifNull(steam, '') != '', assumeNotNull(steam),
        ifNull(eos, '') != '', assumeNotNull(eos),
        assumeNotNull(epic)
    ),
    server_id
SETTINGS max_threads = 2, max_memory_usage = 4000000000;
```

Compare each month with the corresponding raw aggregate before advancing. A
rerun without first removing the month’s rollup partition will double count
SummingMergeTree and aggregate-state data.

After every historical month is verified, publish a new state row. Preserve
the original `created_at`:

```sql
INSERT INTO squad_aegis.sql_rollup_state
SELECT
    rollup_name,
    argMax(created_at, updated_at),
    toNullable(toDate('1970-01-01')),
    1,
    now64(3)
FROM squad_aegis.sql_rollup_state
WHERE rollup_name = 'player_ip_activity'
GROUP BY rollup_name;
```

For a daily rollup, set `first_fully_covered_day` to the earliest verified
backfilled day rather than `1970-01-01`.

## Safe ClickHouse maintenance

Discover maintenance targets from `system.tables`; never accept a caller
provided table name:

```sql
SELECT name
FROM system.tables
WHERE database = 'squad_aegis'
  AND engine IN ('MergeTree', 'ReplacingMergeTree', 'SummingMergeTree', 'AggregatingMergeTree')
ORDER BY name;
```

If optimization is necessary, backtick-quote the discovered name and run
ordinary `OPTIMIZE TABLE` only:

```sql
OPTIMIZE TABLE squad_aegis.`server_event_daily_counts`;
```

Never use `OPTIMIZE TABLE ... FINAL` as a routine application or sudo action.

## Rollback

1. Stop or drain new writers before reversing ClickHouse DDL.
2. Roll back the backend and web client together; cursor and offset APIs are
   intentionally incompatible.
3. Apply ClickHouse down migrations from newest to oldest. The legacy
   `player_identities`, `player_identity_lookup`, and
   `workflow_execution_summary` tables remain available for one
   rollback-compatible release.
4. If only an identity publication is bad, leave the tables in place and
   republish the prior retained snapshot:

   ```sql
   INSERT INTO squad_aegis.player_identity_current
       (pointer_key, snapshot_id, identity_count, lookup_count, published_at)
   SELECT
       1,
       snapshot_id,
       count(),
       (
           SELECT count()
           FROM squad_aegis.player_identity_lookup_snapshots lookup
           WHERE lookup.snapshot_id = identities.snapshot_id
       ),
       now64(3)
   FROM squad_aegis.player_identity_snapshots identities
   WHERE snapshot_id = toUUID('00000000-0000-0000-0000-000000000000')
   GROUP BY snapshot_id;
   ```

5. PostgreSQL down migrations recreate only indexes that existed before this
   release. Apply concurrent down migrations outside a transaction.
6. Recheck both query logs and pool health before restoring traffic.

Do not drop the legacy identity or workflow-summary tables until the
rollback-compatible release window has ended and backups have been verified.
