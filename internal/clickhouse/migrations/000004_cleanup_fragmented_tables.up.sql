DROP TABLE IF EXISTS squad_aegis.server_new_game_events;
--migration:split
DROP TABLE IF EXISTS squad_aegis.server_round_ended_events;
--migration:split
DROP VIEW IF EXISTS squad_aegis.mv_round_ended_events;
DROP VIEW IF EXISTS squad_aegis.mv_new_game_events;
--migration:split
ALTER TABLE squad_aegis.server_game_events_unified 
ADD INDEX IF NOT EXISTS idx_faction faction TYPE set(0) GRANULARITY 64;
--migration:split
ALTER TABLE squad_aegis.server_game_events_unified 
ADD INDEX IF NOT EXISTS idx_map_layer_classname (map_classname, layer_classname) TYPE set(0) GRANULARITY 64;
--migration:split
ALTER TABLE squad_aegis.server_game_events_unified 
ADD INDEX IF NOT EXISTS idx_winner_layer (winner, layer) TYPE set(0) GRANULARITY 64;
