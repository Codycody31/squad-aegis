--migration:split
ALTER TABLE squad_aegis.server_player_connected_events
ADD COLUMN IF NOT EXISTS epic Nullable(String) AFTER eos;

--migration:split
ALTER TABLE squad_aegis.server_player_disconnected_events
ADD COLUMN IF NOT EXISTS epic Nullable(String) AFTER eos;

--migration:split
ALTER TABLE squad_aegis.server_join_succeeded_events
ADD COLUMN IF NOT EXISTS epic Nullable(String) AFTER eos;

--migration:split
ALTER TABLE squad_aegis.server_player_possess_events
ADD COLUMN IF NOT EXISTS player_epic Nullable(String) AFTER player_steam;

--migration:split
ALTER TABLE squad_aegis.player_identities
ADD COLUMN IF NOT EXISTS primary_epic_id String AFTER primary_eos_id,
ADD COLUMN IF NOT EXISTS all_epic_ids Array(String) AFTER all_eos_ids;
