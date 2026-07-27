ALTER TABLE squad_aegis.server_deployable_damaged_events
DROP COLUMN IF EXISTS evidence_key,
DROP COLUMN IF EXISTS cursor_key;

--migration:split

ALTER TABLE squad_aegis.server_player_connected_events
DROP COLUMN IF EXISTS evidence_key,
DROP COLUMN IF EXISTS cursor_key;

--migration:split

ALTER TABLE squad_aegis.server_player_disconnected_events
DROP COLUMN IF EXISTS evidence_key,
DROP COLUMN IF EXISTS cursor_key;

--migration:split

ALTER TABLE squad_aegis.server_join_succeeded_events
DROP COLUMN IF EXISTS evidence_key,
DROP COLUMN IF EXISTS cursor_key;

--migration:split

ALTER TABLE squad_aegis.server_player_possess_events
DROP COLUMN IF EXISTS evidence_key,
DROP COLUMN IF EXISTS cursor_key;

--migration:split

ALTER TABLE squad_aegis.server_player_damaged_events
DROP COLUMN IF EXISTS evidence_key,
DROP COLUMN IF EXISTS cursor_key;

--migration:split

ALTER TABLE squad_aegis.server_player_died_events
DROP COLUMN IF EXISTS evidence_key,
DROP COLUMN IF EXISTS cursor_key;

--migration:split

ALTER TABLE squad_aegis.server_player_wounded_events
DROP COLUMN IF EXISTS evidence_key,
DROP COLUMN IF EXISTS cursor_key;

--migration:split

ALTER TABLE squad_aegis.server_player_revived_events
DROP COLUMN IF EXISTS evidence_key,
DROP COLUMN IF EXISTS cursor_key;
