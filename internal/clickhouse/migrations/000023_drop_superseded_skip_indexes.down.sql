ALTER TABLE squad_aegis.workflow_execution_logs
ADD INDEX IF NOT EXISTS idx_trigger_event_type trigger_event_type
    TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 64,
ADD INDEX IF NOT EXISTS idx_step_name_tokenbf step_name
    TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 64;

--migration:split

ALTER TABLE squad_aegis.workflow_log_messages
ADD INDEX IF NOT EXISTS idx_log_level_tokenbf log_level
    TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 64,
ADD INDEX IF NOT EXISTS idx_message_tokenbf message
    TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 64,
ADD INDEX IF NOT EXISTS idx_step_name_tokenbf step_name
    TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 64;

--migration:split

ALTER TABLE squad_aegis.server_game_events_unified
ADD INDEX IF NOT EXISTS idx_event_type event_type TYPE set(0) GRANULARITY 64,
ADD INDEX IF NOT EXISTS idx_winner winner TYPE set(0) GRANULARITY 64,
ADD INDEX IF NOT EXISTS idx_layer layer TYPE set(0) GRANULARITY 64,
ADD INDEX IF NOT EXISTS idx_faction faction TYPE set(0) GRANULARITY 64,
ADD INDEX IF NOT EXISTS idx_map_layer_classname (map_classname, layer_classname)
    TYPE set(0) GRANULARITY 64,
ADD INDEX IF NOT EXISTS idx_winner_layer (winner, layer)
    TYPE set(0) GRANULARITY 64;
