ALTER TABLE squad_aegis.server_deployable_damaged_events
ADD COLUMN IF NOT EXISTS cursor_key UUID DEFAULT reinterpretAsUUID(MD5(concat(
    toString(server_id), '|', toString(event_time), '|', chain_id, '|',
    deployable, '|', toString(damage), '|', weapon, '|', player_suffix, '|',
    damage_type, '|', toString(health_remaining)
))),
ADD COLUMN IF NOT EXISTS evidence_key UUID ALIAS cursor_key;

--migration:split

ALTER TABLE squad_aegis.server_player_connected_events
ADD COLUMN IF NOT EXISTS cursor_key UUID DEFAULT reinterpretAsUUID(MD5(concat(
    toString(server_id), '|', toString(event_time), '|', chain_id, '|',
    player_controller, '|', ip, '|', ifNull(steam, ''), '|', ifNull(eos, ''), '|',
    ifNull(epic, ''), '|', player_suffix
))),
ADD COLUMN IF NOT EXISTS evidence_key UUID ALIAS cursor_key;

--migration:split

ALTER TABLE squad_aegis.server_player_disconnected_events
ADD COLUMN IF NOT EXISTS cursor_key UUID DEFAULT reinterpretAsUUID(MD5(concat(
    toString(server_id), '|', toString(event_time), '|', chain_id, '|',
    player_controller, '|', player_suffix, '|', ifNull(team, ''), '|', ip, '|',
    ifNull(steam, ''), '|', ifNull(eos, ''), '|', ifNull(epic, '')
))),
ADD COLUMN IF NOT EXISTS evidence_key UUID ALIAS cursor_key;

--migration:split

ALTER TABLE squad_aegis.server_join_succeeded_events
ADD COLUMN IF NOT EXISTS cursor_key UUID DEFAULT reinterpretAsUUID(MD5(concat(
    toString(server_id), '|', toString(event_time), '|', chain_id, '|',
    player_suffix, '|', ifNull(ip, ''), '|', ifNull(steam, ''), '|',
    ifNull(eos, ''), '|', ifNull(epic, '')
))),
ADD COLUMN IF NOT EXISTS evidence_key UUID ALIAS cursor_key;

--migration:split

ALTER TABLE squad_aegis.server_player_possess_events
ADD COLUMN IF NOT EXISTS cursor_key UUID DEFAULT reinterpretAsUUID(MD5(concat(
    toString(server_id), '|', toString(event_time), '|', chain_id, '|',
    player_suffix, '|', possess_classname, '|', ifNull(player_eos, ''), '|',
    ifNull(player_steam, ''), '|', ifNull(player_epic, '')
))),
ADD COLUMN IF NOT EXISTS evidence_key UUID ALIAS cursor_key;

--migration:split

ALTER TABLE squad_aegis.server_player_damaged_events
ADD COLUMN IF NOT EXISTS cursor_key UUID DEFAULT reinterpretAsUUID(MD5(concat(
    toString(server_id), '|', toString(event_time), '|', chain_id, '|',
    victim_name, '|', ifNull(victim_eos, ''), '|', ifNull(victim_steam, ''), '|',
    attacker_name, '|', ifNull(attacker_eos, ''), '|', ifNull(attacker_steam, ''), '|',
    weapon, '|', toString(damage)
))),
ADD COLUMN IF NOT EXISTS evidence_key UUID ALIAS cursor_key;

--migration:split

ALTER TABLE squad_aegis.server_player_died_events
ADD COLUMN IF NOT EXISTS cursor_key UUID DEFAULT reinterpretAsUUID(MD5(concat(
    toString(server_id), '|', toString(event_time), '|', chain_id, '|',
    victim_name, '|', ifNull(victim_eos, ''), '|', ifNull(victim_steam, ''), '|',
    attacker_name, '|', ifNull(attacker_eos, ''), '|', ifNull(attacker_steam, ''), '|',
    weapon, '|', toString(damage), '|', toString(teamkill)
))),
ADD COLUMN IF NOT EXISTS evidence_key UUID ALIAS cursor_key;

--migration:split

ALTER TABLE squad_aegis.server_player_wounded_events
ADD COLUMN IF NOT EXISTS cursor_key UUID DEFAULT reinterpretAsUUID(MD5(concat(
    toString(server_id), '|', toString(event_time), '|', chain_id, '|',
    victim_name, '|', ifNull(victim_eos, ''), '|', ifNull(victim_steam, ''), '|',
    attacker_name, '|', ifNull(attacker_eos, ''), '|', ifNull(attacker_steam, ''), '|',
    weapon, '|', toString(damage), '|', toString(teamkill)
))),
ADD COLUMN IF NOT EXISTS evidence_key UUID ALIAS cursor_key;

--migration:split

ALTER TABLE squad_aegis.server_player_revived_events
ADD COLUMN IF NOT EXISTS cursor_key UUID DEFAULT reinterpretAsUUID(MD5(concat(
    toString(server_id), '|', toString(event_time), '|', chain_id, '|',
    reviver_name, '|', ifNull(reviver_eos, ''), '|', ifNull(reviver_steam, ''), '|',
    victim_name, '|', ifNull(victim_eos, ''), '|', ifNull(victim_steam, '')
))),
ADD COLUMN IF NOT EXISTS evidence_key UUID ALIAS cursor_key;
