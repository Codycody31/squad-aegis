ALTER TABLE squad_aegis.server_player_chat_messages
DROP INDEX IF EXISTS idx_chat_player_bloom,
DROP INDEX IF EXISTS idx_chat_message_ngram;

--migration:split

ALTER TABLE squad_aegis.server_player_connected_events
DROP INDEX IF EXISTS idx_connected_identity_bloom;

--migration:split

ALTER TABLE squad_aegis.server_player_disconnected_events
DROP INDEX IF EXISTS idx_disconnected_identity_bloom;

--migration:split

ALTER TABLE squad_aegis.server_join_succeeded_events
DROP INDEX IF EXISTS idx_join_identity_bloom;

--migration:split

ALTER TABLE squad_aegis.server_player_possess_events
DROP INDEX IF EXISTS idx_possess_identity_bloom;

--migration:split

ALTER TABLE squad_aegis.server_player_died_events
DROP INDEX IF EXISTS idx_died_identity_bloom;

--migration:split

ALTER TABLE squad_aegis.server_player_wounded_events
DROP INDEX IF EXISTS idx_wounded_identity_bloom;

--migration:split

ALTER TABLE squad_aegis.server_player_damaged_events
DROP INDEX IF EXISTS idx_damaged_identity_bloom;

--migration:split

ALTER TABLE squad_aegis.server_player_revived_events
DROP INDEX IF EXISTS idx_revived_identity_bloom;

--migration:split

ALTER TABLE squad_aegis.player_rule_violations
DROP INDEX IF EXISTS idx_rule_violation_identity_bloom;

--migration:split

ALTER TABLE squad_aegis.plugin_logs
DROP INDEX IF EXISTS idx_plugin_text_ngram;
