-- Add victim and attacker details to existing player event tables
--migration:split
-- Enhanced player damaged events table with victim/attacker details
ALTER TABLE squad_aegis.server_player_damaged_events 
ADD COLUMN IF NOT EXISTS victim_eos Nullable(String) AFTER victim_name,
ADD COLUMN IF NOT EXISTS victim_steam Nullable(String) AFTER victim_eos,
ADD COLUMN IF NOT EXISTS victim_team Nullable(String) AFTER victim_steam,
ADD COLUMN IF NOT EXISTS victim_squad Nullable(String) AFTER victim_team,
ADD COLUMN IF NOT EXISTS attacker_team Nullable(String) AFTER attacker_steam,
ADD COLUMN IF NOT EXISTS attacker_squad Nullable(String) AFTER attacker_team,
ADD COLUMN IF NOT EXISTS teamkill UInt8 DEFAULT 0 AFTER damage;

--migration:split
-- Enhanced player died events table with victim/attacker details
ALTER TABLE squad_aegis.server_player_died_events 
ADD COLUMN IF NOT EXISTS victim_eos Nullable(String) AFTER victim_name,
ADD COLUMN IF NOT EXISTS victim_steam Nullable(String) AFTER victim_eos,
ADD COLUMN IF NOT EXISTS victim_team Nullable(String) AFTER victim_steam,
ADD COLUMN IF NOT EXISTS victim_squad Nullable(String) AFTER victim_team,
ADD COLUMN IF NOT EXISTS attacker_name String AFTER attacker_steam,
ADD COLUMN IF NOT EXISTS attacker_team Nullable(String) AFTER attacker_name,
ADD COLUMN IF NOT EXISTS attacker_squad Nullable(String) AFTER attacker_team;

--migration:split
-- Enhanced player wounded events table with victim/attacker details
ALTER TABLE squad_aegis.server_player_wounded_events 
ADD COLUMN IF NOT EXISTS victim_eos Nullable(String) AFTER victim_name,
ADD COLUMN IF NOT EXISTS victim_steam Nullable(String) AFTER victim_eos,
ADD COLUMN IF NOT EXISTS victim_team Nullable(String) AFTER victim_steam,
ADD COLUMN IF NOT EXISTS victim_squad Nullable(String) AFTER victim_team,
ADD COLUMN IF NOT EXISTS attacker_name String AFTER attacker_steam,
ADD COLUMN IF NOT EXISTS attacker_team Nullable(String) AFTER attacker_name,
ADD COLUMN IF NOT EXISTS attacker_squad Nullable(String) AFTER attacker_team,
ADD COLUMN IF NOT EXISTS teamkill UInt8 DEFAULT 0 AFTER damage;

--migration:split
-- Enhanced player revived events table with victim/attacker details
ALTER TABLE squad_aegis.server_player_revived_events 
ADD COLUMN IF NOT EXISTS reviver_team Nullable(String) AFTER reviver_steam,
ADD COLUMN IF NOT EXISTS reviver_squad Nullable(String) AFTER reviver_team,
ADD COLUMN IF NOT EXISTS victim_team Nullable(String) AFTER victim_steam,
ADD COLUMN IF NOT EXISTS victim_squad Nullable(String) AFTER victim_team,
ADD COLUMN IF NOT EXISTS teamkill UInt8 DEFAULT 0 AFTER victim_name;
