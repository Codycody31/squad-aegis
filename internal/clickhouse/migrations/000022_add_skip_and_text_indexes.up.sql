ALTER TABLE squad_aegis.server_player_chat_messages
ADD INDEX IF NOT EXISTS idx_chat_player_bloom (steam_id, eos_id)
    TYPE bloom_filter(0.01) GRANULARITY 1,
ADD INDEX IF NOT EXISTS idx_chat_message_ngram lowerUTF8(message)
    TYPE ngrambf_v1(3, 10000, 3, 0) GRANULARITY 1;

--migration:split

ALTER TABLE squad_aegis.server_player_connected_events
ADD INDEX IF NOT EXISTS idx_connected_identity_bloom (steam, eos, epic)
    TYPE bloom_filter(0.01) GRANULARITY 1;

--migration:split

ALTER TABLE squad_aegis.server_player_disconnected_events
ADD INDEX IF NOT EXISTS idx_disconnected_identity_bloom (steam, eos, epic)
    TYPE bloom_filter(0.01) GRANULARITY 1;

--migration:split

ALTER TABLE squad_aegis.server_join_succeeded_events
ADD INDEX IF NOT EXISTS idx_join_identity_bloom (steam, eos, epic)
    TYPE bloom_filter(0.01) GRANULARITY 1;

--migration:split

ALTER TABLE squad_aegis.server_player_possess_events
ADD INDEX IF NOT EXISTS idx_possess_identity_bloom (player_steam, player_eos, player_epic)
    TYPE bloom_filter(0.01) GRANULARITY 1;

--migration:split

ALTER TABLE squad_aegis.server_player_died_events
ADD INDEX IF NOT EXISTS idx_died_identity_bloom
    (victim_steam, victim_eos, attacker_steam, attacker_eos)
    TYPE bloom_filter(0.01) GRANULARITY 1;

--migration:split

ALTER TABLE squad_aegis.server_player_wounded_events
ADD INDEX IF NOT EXISTS idx_wounded_identity_bloom
    (victim_steam, victim_eos, attacker_steam, attacker_eos)
    TYPE bloom_filter(0.01) GRANULARITY 1;

--migration:split

ALTER TABLE squad_aegis.server_player_damaged_events
ADD INDEX IF NOT EXISTS idx_damaged_identity_bloom
    (victim_steam, victim_eos, attacker_steam, attacker_eos)
    TYPE bloom_filter(0.01) GRANULARITY 1;

--migration:split

ALTER TABLE squad_aegis.server_player_revived_events
ADD INDEX IF NOT EXISTS idx_revived_identity_bloom
    (victim_steam, victim_eos, reviver_steam, reviver_eos)
    TYPE bloom_filter(0.01) GRANULARITY 1;

--migration:split

ALTER TABLE squad_aegis.player_rule_violations
ADD INDEX IF NOT EXISTS idx_rule_violation_identity_bloom (player_id, player_eos_id)
    TYPE bloom_filter(0.01) GRANULARITY 1;

--migration:split

ALTER TABLE squad_aegis.plugin_logs
ADD INDEX IF NOT EXISTS idx_plugin_text_ngram
    lowerUTF8(concat(message, ' ', ifNull(error_message, '')))
    TYPE ngrambf_v1(3, 10000, 3, 0) GRANULARITY 1;
