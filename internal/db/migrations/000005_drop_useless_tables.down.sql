-- Reverse migration for PostgreSQL steam_id simplification
-- WARNING: This migration cannot restore player data that was moved to ClickHouse
-- Manual data restoration would be required

-- Step 1: Recreate the players table
CREATE TABLE IF NOT EXISTS "public"."players" (
    "id" uuid NOT NULL,
    "steam_id" bigint NOT NULL UNIQUE,
    "display_name" varchar,
    "avatar_url" varchar,
    "first_seen" timestamp NOT NULL,
    "last_seen" timestamp NOT NULL,
    PRIMARY KEY ("id")
);

CREATE INDEX IF NOT EXISTS "idx_players_steam_id" ON "public"."players" ("steam_id");
CREATE INDEX IF NOT EXISTS "idx_players_last_seen" ON "public"."players" ("last_seen");

-- Step 2: Recreate the dropped tables
CREATE TABLE IF NOT EXISTS "public"."integrations" (
    "id" uuid NOT NULL,
    "server_id" uuid,
    "slug" varchar NOT NULL,
    "type" integer NOT NULL,
    "enabled" boolean NOT NULL,
    "config" jsonb,
    "notes" text,
    "created_at" timestamp NOT NULL,
    "updated_at" timestamp NOT NULL,
    PRIMARY KEY ("id")
);

CREATE TABLE IF NOT EXISTS "public"."play_sessions" (
    "id" bigint NOT NULL,
    "player_id" uuid NOT NULL,
    "server_id" uuid NOT NULL,
    "connected_at" timestamp NOT NULL,
    "disconnected_at" timestamp,
    "duration_sec" integer,
    PRIMARY KEY ("id")
);

CREATE TABLE IF NOT EXISTS "public"."server_players" (
    "id" uuid NOT NULL,
    "player_id" uuid NOT NULL,
    "server_id" uuid NOT NULL,
    "first_seen" timestamp NOT NULL,
    "last_seen" timestamp NOT NULL,
    PRIMARY KEY ("id")
);

CREATE TABLE IF NOT EXISTS "public"."server_player_chat_messages" (
    "id" uuid NOT NULL,
    "player_id" uuid NOT NULL,
    "server_id" uuid NOT NULL,
    "type" bigint NOT NULL,
    "message" text NOT NULL,
    "sent_at" timestamp NOT NULL,
    PRIMARY KEY ("id")
);

-- Step 3: Add player_id column back to server_bans
ALTER TABLE server_bans ADD COLUMN IF NOT EXISTS player_id uuid;

-- NOTE: Cannot automatically restore player_id references from steam_id
-- Manual data migration would be required to populate player_id values

-- Step 4: Recreate foreign key constraints
ALTER TABLE "public"."integrations"
ADD CONSTRAINT "fk_integrations_server_id_servers_id" FOREIGN KEY("server_id") REFERENCES "public"."servers"("id");

ALTER TABLE "public"."play_sessions"
ADD CONSTRAINT "fk_play_sessions_player_id_players_id" FOREIGN KEY("player_id") REFERENCES "public"."players"("id");

ALTER TABLE "public"."play_sessions"
ADD CONSTRAINT "fk_play_sessions_server_id_servers_id" FOREIGN KEY("server_id") REFERENCES "public"."servers"("id");

ALTER TABLE "public"."server_player_chat_messages"
ADD CONSTRAINT "fk_server_player_chat_messages_player_id_players_id" FOREIGN KEY("player_id") REFERENCES "public"."players"("id");

ALTER TABLE "public"."server_player_chat_messages"
ADD CONSTRAINT "fk_server_player_chat_messages_server_id_servers_id" FOREIGN KEY("server_id") REFERENCES "public"."servers"("id");

ALTER TABLE "public"."server_players"
ADD CONSTRAINT "fk_server_players_player_id_players_id" FOREIGN KEY("player_id") REFERENCES "public"."players"("id");

ALTER TABLE "public"."server_players"
ADD CONSTRAINT "fk_server_players_server_id_servers_id" FOREIGN KEY("server_id") REFERENCES "public"."servers"("id");

-- Recreate indexes
CREATE INDEX IF NOT EXISTS "idx_play_sessions_player_id" ON "public"."play_sessions" ("player_id");
CREATE INDEX IF NOT EXISTS "idx_play_sessions_server_id" ON "public"."play_sessions" ("server_id");
CREATE INDEX IF NOT EXISTS "idx_play_sessions_connected_at" ON "public"."play_sessions" ("connected_at");

CREATE INDEX IF NOT EXISTS "idx_server_players_server_id" ON "public"."server_players" ("server_id");
CREATE INDEX IF NOT EXISTS "idx_server_players_player_id" ON "public"."server_players" ("player_id");

CREATE INDEX IF NOT EXISTS "idx_server_player_chat_messages_server_id" ON "public"."server_player_chat_messages" ("server_id");
CREATE INDEX IF NOT EXISTS "idx_server_player_chat_messages_player_id" ON "public"."server_player_chat_messages" ("player_id");
CREATE INDEX IF NOT EXISTS "idx_server_player_chat_messages_sent_at" ON "public"."server_player_chat_messages" ("sent_at");

-- Step 5: Optionally remove steam_id from server_bans (WARNING: Data loss)
-- Uncomment the following lines to restore original server_bans structure
-- Note: This will lose the steam_id data which is important for player identification
-- ALTER TABLE server_bans DROP COLUMN IF EXISTS steam_id;
-- DROP INDEX IF EXISTS idx_server_bans_steam_id;

-- WARNING: The above operations would cause significant data loss
-- Recommend keeping steam_id column for compatibility
