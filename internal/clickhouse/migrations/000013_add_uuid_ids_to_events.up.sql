--migration:split
-- Add UUID id column to server_deployable_damaged_events
ALTER TABLE squad_aegis.server_deployable_damaged_events 
ADD COLUMN IF NOT EXISTS id UUID DEFAULT generateUUIDv4();

--migration:split
-- Add UUID id column to server_player_connected_events
ALTER TABLE squad_aegis.server_player_connected_events 
ADD COLUMN IF NOT EXISTS id UUID DEFAULT generateUUIDv4();

--migration:split
-- Add UUID id column to server_player_damaged_events
ALTER TABLE squad_aegis.server_player_damaged_events 
ADD COLUMN IF NOT EXISTS id UUID DEFAULT generateUUIDv4();

--migration:split
-- Add UUID id column to server_player_died_events
ALTER TABLE squad_aegis.server_player_died_events 
ADD COLUMN IF NOT EXISTS id UUID DEFAULT generateUUIDv4();

--migration:split
-- Add UUID id column to server_join_succeeded_events
ALTER TABLE squad_aegis.server_join_succeeded_events 
ADD COLUMN IF NOT EXISTS id UUID DEFAULT generateUUIDv4();

--migration:split
-- Add UUID id column to server_player_possess_events
ALTER TABLE squad_aegis.server_player_possess_events 
ADD COLUMN IF NOT EXISTS id UUID DEFAULT generateUUIDv4();

--migration:split
-- Add UUID id column to server_player_revived_events
ALTER TABLE squad_aegis.server_player_revived_events 
ADD COLUMN IF NOT EXISTS id UUID DEFAULT generateUUIDv4();

--migration:split
-- Add UUID id column to server_player_wounded_events
ALTER TABLE squad_aegis.server_player_wounded_events 
ADD COLUMN IF NOT EXISTS id UUID DEFAULT generateUUIDv4();

--migration:split
-- Add UUID id column to server_player_disconnected_events
ALTER TABLE squad_aegis.server_player_disconnected_events 
ADD COLUMN IF NOT EXISTS id UUID DEFAULT generateUUIDv4();