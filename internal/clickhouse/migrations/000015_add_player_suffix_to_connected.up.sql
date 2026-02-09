-- Add player_suffix to server_player_connected_events for historical/evidence tracking
-- (live feed already emits player_suffix; this enables persistence and search)
ALTER TABLE squad_aegis.server_player_connected_events
ADD COLUMN IF NOT EXISTS player_suffix String DEFAULT '';
