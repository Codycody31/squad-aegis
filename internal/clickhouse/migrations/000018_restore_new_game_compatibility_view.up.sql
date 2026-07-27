CREATE VIEW IF NOT EXISTS squad_aegis.server_new_game_events AS
SELECT
    event_time,
    server_id,
    chain_id,
    toString(team) AS team,
    subfaction,
    faction,
    action,
    toString(tickets) AS tickets,
    layer,
    level,
    dlc,
    map_classname,
    layer_classname,
    ingested_at
FROM squad_aegis.server_game_events_unified
WHERE event_type = 'NEW_GAME';
