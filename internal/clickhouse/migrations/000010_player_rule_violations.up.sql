CREATE TABLE IF NOT EXISTS squad_aegis.player_rule_violations (
    violation_id UUID DEFAULT generateUUIDv4(),
    server_id UUID NOT NULL,
    player_steam_id UInt64 NOT NULL,
    rule_id Nullable(UUID),
    admin_user_id Nullable(UUID),
    action_type LowCardinality(String) NOT NULL, -- 'WARN', 'KICK', 'BAN'
    created_at DateTime64(3, 'UTC') NOT NULL,
    ingested_at DateTime DEFAULT now()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (server_id, created_at, player_steam_id, violation_id)
SETTINGS index_granularity = 8192;

