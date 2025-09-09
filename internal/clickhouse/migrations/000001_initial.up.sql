CREATE DATABASE IF NOT EXISTS squad_aegis;

--migration:split
CREATE TABLE IF NOT EXISTS squad_aegis.server_player_chat_messages (
    message_id  UUID DEFAULT generateUUIDv4(),
    server_id   UUID,
    player_name String,
    steam_id    UInt64,                     -- use steam_id instead of player_id
    eos_id      String,
    sent_at     DateTime64(3, 'UTC'),
    chat_type        String,                   -- e.g. "all", "team", "squad", "admin"
    message     String CODEC(ZSTD(5)),
    ingest_ts   DateTime DEFAULT now(),
    INDEX idx_msg_tokenbf message TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 64
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(sent_at)
ORDER BY (server_id, sent_at, steam_id, message_id);

--migration:split
CREATE TABLE IF NOT EXISTS squad_aegis.server_admin_broadcast_events (
    event_time  DateTime64(3, 'UTC'),
    server_id   UUID,
    chain_id    String,
    message     String,
    from_user   String,
    ingested_at DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (server_id, event_time);

--migration:split
CREATE TABLE IF NOT EXISTS squad_aegis.server_deployable_damaged_events (
    event_time        DateTime64(3, 'UTC'),
    server_id         UUID,
    chain_id          String,
    deployable        LowCardinality(String),
    damage            Float32,
    weapon            LowCardinality(String),
    player_suffix     String,
    damage_type       LowCardinality(String),
    health_remaining  Float32,
    ingested_at       DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (server_id, event_time);

--migration:split
CREATE TABLE IF NOT EXISTS squad_aegis.server_tick_rate_events (
    event_time  DateTime64(3, 'UTC'),
    server_id   UUID,
    chain_id    String,
    tick_rate   Float32,
    ingested_at DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (server_id, event_time);

--migration:split
CREATE TABLE IF NOT EXISTS squad_aegis.server_player_connected_events (
    event_time         DateTime64(3, 'UTC'),
    server_id          UUID,
    chain_id           String,
    player_controller  String,
    ip                 String,
    steam              Nullable(String),
    eos                Nullable(String),
    ingested_at        DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (server_id, event_time);

--migration:split
CREATE TABLE IF NOT EXISTS squad_aegis.server_new_game_events (
    event_time      DateTime64(3, 'UTC'),
    server_id       UUID,
    chain_id        String,
    team            Nullable(String),
    subfaction      Nullable(String),
    faction         Nullable(String),
    action          Nullable(String),
    tickets         Nullable(String),
    layer           Nullable(String),
    level           Nullable(String),
    dlc             Nullable(String),
    map_classname   Nullable(String),
    layer_classname Nullable(String),
    ingested_at     DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (server_id, event_time);

--migration:split
CREATE TABLE IF NOT EXISTS squad_aegis.server_player_damaged_events (
    event_time            DateTime64(3, 'UTC'),
    server_id             UUID,
    chain_id              String,
    victim_name           String,
    damage                Float32,
    attacker_name         String,
    attacker_controller   String,
    weapon                LowCardinality(String),
    attacker_eos          Nullable(String),
    attacker_steam        Nullable(String),
    ingested_at           DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (server_id, event_time);

--migration:split
CREATE TABLE IF NOT EXISTS squad_aegis.server_player_died_events (
    event_time                 DateTime64(3, 'UTC'),
    wound_time                 Nullable(DateTime64(3, 'UTC')),
    server_id                  UUID,
    chain_id                   String,
    victim_name                String,
    damage                     Float32,
    attacker_player_controller String,
    weapon                     LowCardinality(String),
    attacker_eos               Nullable(String),
    attacker_steam             Nullable(String),
    teamkill                   UInt8 DEFAULT 0,
    ingested_at                DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (server_id, event_time);

--migration:split
CREATE TABLE IF NOT EXISTS squad_aegis.server_join_succeeded_events (
    event_time    DateTime64(3, 'UTC'),
    server_id     UUID,
    chain_id      String,
    player_suffix String,
    ip            Nullable(String),
    steam         Nullable(String),
    eos           Nullable(String),
    ingested_at   DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (server_id, event_time);

--migration:split
CREATE TABLE IF NOT EXISTS squad_aegis.server_player_possess_events (
    event_time         DateTime64(3, 'UTC'),
    server_id          UUID,
    chain_id           String,
    player_suffix      String,
    possess_classname  LowCardinality(String),
    player_eos         Nullable(String),
    player_steam       Nullable(String),
    ingested_at        DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (server_id, event_time);

--migration:split
CREATE TABLE IF NOT EXISTS squad_aegis.server_player_revived_events (
    event_time    DateTime64(3, 'UTC'),
    server_id     UUID,
    chain_id      String,
    reviver_name  String,
    victim_name   String,
    reviver_eos   Nullable(String),
    reviver_steam Nullable(String),
    victim_eos    Nullable(String),
    victim_steam  Nullable(String),
    ingested_at   DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (server_id, event_time);

--migration:split
CREATE TABLE IF NOT EXISTS squad_aegis.server_player_wounded_events (
    event_time                 DateTime64(3, 'UTC'),
    server_id                  UUID,
    chain_id                   String,
    victim_name                String,
    damage                     Float32,
    attacker_player_controller String,
    weapon                     LowCardinality(String),
    attacker_eos               Nullable(String),
    attacker_steam             Nullable(String),
    teamkill                   UInt8 DEFAULT 0,
    ingested_at                DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (server_id, event_time);

--migration:split
CREATE TABLE IF NOT EXISTS squad_aegis.server_round_ended_events (
    event_time  DateTime64(3, 'UTC'),
    server_id   UUID,
    chain_id    Nullable(String),
    winner      Nullable(String),
    layer       Nullable(String),
    winner_json Nullable(String),
    loser_json  Nullable(String),
    ingested_at DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (server_id, event_time);
