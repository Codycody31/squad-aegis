ALTER TABLE squad_aegis.server_player_connected_events
ADD COLUMN IF NOT EXISTS player_suffix String DEFAULT '';
