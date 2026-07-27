ALTER TABLE squad_aegis.workflow_execution_logs
DROP INDEX IF EXISTS idx_trigger_event_type,
DROP INDEX IF EXISTS idx_step_name_tokenbf;

--migration:split

ALTER TABLE squad_aegis.workflow_log_messages
DROP INDEX IF EXISTS idx_log_level_tokenbf,
DROP INDEX IF EXISTS idx_message_tokenbf,
DROP INDEX IF EXISTS idx_step_name_tokenbf;

--migration:split

ALTER TABLE squad_aegis.server_game_events_unified
DROP INDEX IF EXISTS idx_event_type,
DROP INDEX IF EXISTS idx_winner,
DROP INDEX IF EXISTS idx_layer,
DROP INDEX IF EXISTS idx_faction,
DROP INDEX IF EXISTS idx_map_layer_classname,
DROP INDEX IF EXISTS idx_winner_layer;
