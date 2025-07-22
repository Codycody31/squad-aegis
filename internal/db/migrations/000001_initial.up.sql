CREATE SCHEMA IF NOT EXISTS "public";


CREATE TABLE "public"."audit_logs" (
    "id" uuid NOT NULL,
    "server_id" uuid,
    "user_id" uuid,
    "action" character varying(500) NOT NULL,
    "changes" jsonb,
    "timestamp" timestamp without time zone NOT NULL,
    PRIMARY KEY ("id")
);



CREATE TABLE "public"."server_roles" (
    "id" uuid NOT NULL,
    "server_id" uuid NOT NULL,
    "name" character varying(500) NOT NULL,
    "permissions" text NOT NULL,
    "created_at" timestamp without time zone NOT NULL,
    PRIMARY KEY ("id")
);



CREATE TABLE "public"."server_rules" (
    "id" uuid NOT NULL,
    "server_id" uuid NOT NULL,
    "parent_id" uuid,
    "name" varchar NOT NULL,
    "order_key" varchar,
    "description" varchar,
    "ban_duration" bigint,
    PRIMARY KEY ("id")
);



CREATE TABLE "public"."players" (
    "id" uuid NOT NULL,
    "steam_id" bigint NOT NULL UNIQUE,
    "display_name" varchar,
    "avatar_url" varchar,
    "first_seen" timestamp NOT NULL,
    "last_seen" timestamp NOT NULL,
    PRIMARY KEY ("id")
);



CREATE TABLE "public"."server_bans" (
    "id" uuid NOT NULL,
    "server_id" uuid NOT NULL,
    "admin_id" uuid NOT NULL,
    "player_id" uuid NOT NULL,
    "reason" text NOT NULL,
    "duration" integer NOT NULL,
    "rule_id" uuid,
    "created_at" timestamp without time zone NOT NULL,
    "updated_at" timestamp without time zone NOT NULL,
    PRIMARY KEY ("id")
);



CREATE TABLE "public"."integrations" (
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



CREATE TABLE "public"."play_sessions" (
    "id" bigint NOT NULL,
    "player_id" uuid NOT NULL,
    "server_id" uuid NOT NULL,
    "connected_at" timestamp NOT NULL,
    "disconnected_at" timestamp,
    "duration_sec" integer,
    PRIMARY KEY ("id")
);



CREATE TABLE "public"."server_players" (
    "id" uuid NOT NULL,
    "player_id" uuid NOT NULL,
    "server_id" uuid NOT NULL,
    "first_seen" timestamp NOT NULL,
    "last_seen" timestamp NOT NULL,
    PRIMARY KEY ("id")
);



CREATE TABLE "public"."server_player_chat_messages" (
    "id" uuid NOT NULL,
    "player_id" uuid NOT NULL,
    "server_id" uuid NOT NULL,
    "type" bigint NOT NULL,
    "message" text NOT NULL,
    "sent_at" timestamp NOT NULL,
    PRIMARY KEY ("id")
);



CREATE TABLE "public"."servers" (
    "id" uuid NOT NULL,
    "name" character varying(500) NOT NULL,
    "ip_address" inet NOT NULL,
    "game_port" integer NOT NULL,
    "rcon_port" integer NOT NULL,
    "rcon_password" character varying(500) NOT NULL,
    "created_at" timestamp without time zone NOT NULL,
    "updated_at" timestamp without time zone NOT NULL,
    PRIMARY KEY ("id")
);



CREATE TABLE "public"."users" (
    "id" uuid NOT NULL,
    "steam_id" bigint,
    "name" character varying(500) NOT NULL,
    "username" character varying(500) NOT NULL UNIQUE,
    "password" character varying(500) NOT NULL,
    "super_admin" boolean NOT NULL,
    "created_at" timestamp without time zone NOT NULL,
    "updated_at" timestamp without time zone NOT NULL,
    PRIMARY KEY ("id")
);


CREATE UNIQUE INDEX "users_users_username_key"
ON "public"."users" ("username");


CREATE TABLE "public"."sessions" (
    "id" uuid NOT NULL,
    "user_id" uuid NOT NULL,
    "token" text NOT NULL UNIQUE,
    "created_at" timestamp without time zone NOT NULL,
    "expires_at" timestamp without time zone,
    "last_seen" timestamp without time zone NOT NULL,
    "last_seen_ip" character varying(500) NOT NULL,
    PRIMARY KEY ("id")
);


CREATE UNIQUE INDEX "sessions_sessions_token_key"
ON "public"."sessions" ("token");


CREATE TABLE "public"."server_admins" (
    "id" uuid NOT NULL,
    "server_id" uuid NOT NULL,
    "user_id" uuid,
    "steam_id" bigint,
    "server_role_id" uuid NOT NULL,
    "created_at" timestamp without time zone NOT NULL,
    PRIMARY KEY ("id")
);



ALTER TABLE "public"."audit_logs"
ADD CONSTRAINT "fk_audit_logs_server_id_servers_id" FOREIGN KEY("server_id") REFERENCES "public"."servers"("id");

ALTER TABLE "public"."audit_logs"
ADD CONSTRAINT "fk_audit_logs_user_id_users_id" FOREIGN KEY("user_id") REFERENCES "public"."users"("id");

ALTER TABLE "public"."integrations"
ADD CONSTRAINT "fk_integrations_server_id_servers_id" FOREIGN KEY("server_id") REFERENCES "public"."servers"("id");

ALTER TABLE "public"."play_sessions"
ADD CONSTRAINT "fk_play_sessions_player_id_players_id" FOREIGN KEY("player_id") REFERENCES "public"."players"("id");

ALTER TABLE "public"."play_sessions"
ADD CONSTRAINT "fk_play_sessions_server_id_servers_id" FOREIGN KEY("server_id") REFERENCES "public"."servers"("id");

ALTER TABLE "public"."server_admins"
ADD CONSTRAINT "fk_server_admins_server_id_servers_id" FOREIGN KEY("server_id") REFERENCES "public"."servers"("id");

ALTER TABLE "public"."server_admins"
ADD CONSTRAINT "fk_server_admins_user_id_users_id" FOREIGN KEY("user_id") REFERENCES "public"."users"("id");

ALTER TABLE "public"."server_bans"
ADD CONSTRAINT "fk_server_bans_admin_id_users_id" FOREIGN KEY("admin_id") REFERENCES "public"."users"("id");

ALTER TABLE "public"."server_bans"
ADD CONSTRAINT "fk_server_bans_rule_id_server_rules_id" FOREIGN KEY("rule_id") REFERENCES "public"."server_rules"("id");

ALTER TABLE "public"."server_bans"
ADD CONSTRAINT "fk_server_bans_server_id_servers_id" FOREIGN KEY("server_id") REFERENCES "public"."servers"("id");

ALTER TABLE "public"."server_player_chat_messages"
ADD CONSTRAINT "fk_server_player_chat_messages_player_id_players_id" FOREIGN KEY("player_id") REFERENCES "public"."players"("id");

ALTER TABLE "public"."server_player_chat_messages"
ADD CONSTRAINT "fk_server_player_chat_messages_server_id_servers_id" FOREIGN KEY("server_id") REFERENCES "public"."servers"("id");

ALTER TABLE "public"."server_players"
ADD CONSTRAINT "fk_server_players_player_id_players_id" FOREIGN KEY("player_id") REFERENCES "public"."players"("id");

ALTER TABLE "public"."server_players"
ADD CONSTRAINT "fk_server_players_server_id_servers_id" FOREIGN KEY("server_id") REFERENCES "public"."servers"("id");

ALTER TABLE "public"."server_roles"
ADD CONSTRAINT "fk_server_roles_server_id_servers_id" FOREIGN KEY("server_id") REFERENCES "public"."servers"("id");

ALTER TABLE "public"."server_rules"
ADD CONSTRAINT "fk_server_rules_server_id_servers_id" FOREIGN KEY("server_id") REFERENCES "public"."servers"("id");

ALTER TABLE "public"."sessions"
ADD CONSTRAINT "fk_sessions_user_id_users_id" FOREIGN KEY("user_id") REFERENCES "public"."users"("id");



CREATE INDEX "idx_audit_logs_server_id" ON "public"."audit_logs" ("server_id");
CREATE INDEX "idx_audit_logs_user_id" ON "public"."audit_logs" ("user_id");
CREATE INDEX "idx_audit_logs_timestamp" ON "public"."audit_logs" ("timestamp");

CREATE INDEX "idx_players_steam_id" ON "public"."players" ("steam_id");
CREATE INDEX "idx_players_last_seen" ON "public"."players" ("last_seen");

CREATE INDEX "idx_play_sessions_player_id" ON "public"."play_sessions" ("player_id");
CREATE INDEX "idx_play_sessions_server_id" ON "public"."play_sessions" ("server_id");
CREATE INDEX "idx_play_sessions_connected_at" ON "public"."play_sessions" ("connected_at");

CREATE INDEX "idx_server_players_server_id" ON "public"."server_players" ("server_id");
CREATE INDEX "idx_server_players_player_id" ON "public"."server_players" ("player_id");

CREATE INDEX "idx_server_player_chat_messages_server_id" ON "public"."server_player_chat_messages" ("server_id");
CREATE INDEX "idx_server_player_chat_messages_player_id" ON "public"."server_player_chat_messages" ("player_id");
CREATE INDEX "idx_server_player_chat_messages_sent_at" ON "public"."server_player_chat_messages" ("sent_at");

CREATE INDEX "idx_server_bans_server_id" ON "public"."server_bans" ("server_id");
CREATE INDEX "idx_server_bans_player_id" ON "public"."server_bans" ("player_id");
CREATE INDEX "idx_server_bans_admin_id" ON "public"."server_bans" ("admin_id");
CREATE INDEX "idx_server_bans_created_at" ON "public"."server_bans" ("created_at");

CREATE INDEX "idx_server_admins_server_id" ON "public"."server_admins" ("server_id");
CREATE INDEX "idx_server_admins_user_id" ON "public"."server_admins" ("user_id");
CREATE INDEX "idx_server_admins_steam_id" ON "public"."server_admins" ("steam_id");

CREATE INDEX "idx_sessions_user_id" ON "public"."sessions" ("user_id");
CREATE INDEX "idx_sessions_expires_at" ON "public"."sessions" ("expires_at");