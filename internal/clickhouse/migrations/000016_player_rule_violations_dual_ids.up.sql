CREATE TABLE IF NOT EXISTS squad_aegis.player_rule_violations_v2 (
    violation_id UUID DEFAULT generateUUIDv4(),
    server_id UUID NOT NULL,
    player_id String NOT NULL,
    player_steam_id Nullable(UInt64),
    player_eos_id Nullable(String),
    rule_id Nullable(UUID),
    admin_user_id Nullable(UUID),
    action_type LowCardinality(String) NOT NULL,
    created_at DateTime64(3, 'UTC') NOT NULL,
    ingested_at DateTime DEFAULT now()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (server_id, created_at, player_id, violation_id)
SETTINGS index_granularity = 8192;

INSERT INTO squad_aegis.player_rule_violations_v2
SELECT
    violation_id,
    server_id,
    toString(player_steam_id) AS player_id,
    CAST(player_steam_id, 'Nullable(UInt64)') AS player_steam_id,
    NULL AS player_eos_id,
    rule_id,
    admin_user_id,
    action_type,
    created_at,
    ingested_at
FROM squad_aegis.player_rule_violations;

RENAME TABLE squad_aegis.player_rule_violations TO squad_aegis.player_rule_violations_legacy_000016,
             squad_aegis.player_rule_violations_v2 TO squad_aegis.player_rule_violations;

DROP TABLE IF EXISTS squad_aegis.player_rule_violations_legacy_000016;
