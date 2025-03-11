CREATE SCHEMA IF NOT EXISTS public;

CREATE TABLE IF NOT EXISTS public.users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    steam_id bigint,
    name varchar(500) NOT NULL,
    username varchar(500) NOT NULL UNIQUE,
    password varchar(500) NOT NULL,
    super_admin boolean NOT NULL DEFAULT false,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS public.sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    user_id uuid NOT NULL,
    token text NOT NULL UNIQUE,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at timestamp,
    last_seen timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_ip varchar(500) NOT NULL
);

CREATE TABLE IF NOT EXISTS public.audit_logs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    server_id uuid,
    user_id uuid,
    action varchar(500) NOT NULL,
    changes jsonb,
    timestamp timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS public.servers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    name varchar(500) NOT NULL,
    ip_address inet NOT NULL,
    game_port integer NOT NULL,
    rcon_port integer NOT NULL,
    rcon_password varchar(500) NOT NULL,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS public.server_bans (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    server_id uuid NOT NULL,
    admin_id uuid NOT NULL,
    steam_id bigint NOT NULL,
    reason text NOT NULL,
    duration integer NOT NULL,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS public.server_admins (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    server_id uuid NOT NULL,
    user_id uuid NOT NULL,
    server_role_id uuid NOT NULL,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS public.server_roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    server_id uuid NOT NULL,
    name varchar(500) NOT NULL,
    permissions text NOT NULL,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);
