-- Remove indices
DROP INDEX IF EXISTS server_ban_list_subscriptions_server_id_idx;
DROP INDEX IF EXISTS server_ban_list_subscriptions_ban_list_id_idx;
DROP INDEX IF EXISTS server_bans_ban_list_id_idx;

-- Remove ban_list_id column from server_bans
ALTER TABLE server_bans DROP COLUMN IF EXISTS ban_list_id;

-- Drop server_ban_list_subscriptions table
DROP TABLE IF EXISTS server_ban_list_subscriptions;

-- Drop ban_lists table
DROP TABLE IF EXISTS ban_lists; 