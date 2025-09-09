-- Migrate PostgreSQL to use steam_id consistently instead of player_id references
-- This simplifies the schema by removing the players table and using steam_id directly

-- Step 1: Modify server_bans to use steam_id instead of player_id
-- Add steam_id column if it doesn't exist
ALTER TABLE server_bans ADD COLUMN IF NOT EXISTS steam_id BIGINT;

-- Migrate existing player_id references to steam_id values
UPDATE server_bans 
SET steam_id = (
    SELECT p.steam_id 
    FROM players p 
    WHERE p.id = server_bans.player_id
) 
WHERE steam_id IS NULL AND player_id IS NOT NULL;

-- Make steam_id NOT NULL and add index
ALTER TABLE server_bans ALTER COLUMN steam_id SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_server_bans_steam_id ON server_bans (steam_id);

-- Step 2: Remove foreign key constraints that reference the players table
ALTER TABLE server_bans DROP CONSTRAINT IF EXISTS fk_server_bans_player_id_players_id;
ALTER TABLE play_sessions DROP CONSTRAINT IF EXISTS fk_play_sessions_player_id_players_id;
ALTER TABLE server_players DROP CONSTRAINT IF EXISTS fk_server_players_player_id_players_id;
ALTER TABLE server_player_chat_messages DROP CONSTRAINT IF EXISTS fk_server_player_chat_messages_player_id_players_id;

-- Step 3: Drop the player_id column from server_bans (no longer needed)
ALTER TABLE server_bans DROP COLUMN IF EXISTS player_id;

-- Step 4: Drop all player-related tables - data should be tracked in ClickHouse
DROP TABLE IF EXISTS integrations;
DROP TABLE IF EXISTS server_player_chat_messages;
DROP TABLE IF EXISTS server_players;
DROP TABLE IF EXISTS play_sessions;
DROP TABLE IF EXISTS players;