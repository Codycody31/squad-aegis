CREATE TABLE IF NOT EXISTS public.ban_lists (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar(500) NOT NULL UNIQUE,
    description text,
    is_remote boolean NOT NULL DEFAULT false,
    remote_url text,
    remote_sync_enabled boolean NOT NULL DEFAULT false,
    last_synced_at timestamp,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);
ALTER TABLE public.server_bans
ADD COLUMN IF NOT EXISTS ban_list_id uuid;
CREATE TABLE IF NOT EXISTS public.server_ban_list_subscriptions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id uuid NOT NULL,
    ban_list_id uuid NOT NULL,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (server_id, ban_list_id),
    FOREIGN KEY (server_id) REFERENCES public.servers(id) ON DELETE CASCADE,
    FOREIGN KEY (ban_list_id) REFERENCES public.ban_lists(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS public.remote_ban_sources (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar(500) NOT NULL UNIQUE,
    url text NOT NULL,
    sync_enabled boolean NOT NULL DEFAULT true,
    sync_interval_minutes integer NOT NULL DEFAULT 60,
    last_synced_at timestamp,
    last_sync_status varchar(100),
    last_sync_error text,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);
ALTER TABLE public.server_bans
ADD CONSTRAINT fk_server_bans_ban_list_id_ban_lists_id FOREIGN KEY (ban_list_id) REFERENCES public.ban_lists(id) ON DELETE
SET NULL;
CREATE TABLE IF NOT EXISTS public.ignored_steam_ids (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    steam_id varchar(20) NOT NULL UNIQUE,
    reason text,
    created_by varchar(500),
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_ban_lists_name ON public.ban_lists (name);
CREATE INDEX IF NOT EXISTS idx_server_bans_ban_list_id ON public.server_bans (ban_list_id);
CREATE INDEX IF NOT EXISTS idx_server_ban_list_subscriptions_server_id ON public.server_ban_list_subscriptions (server_id);
CREATE INDEX IF NOT EXISTS idx_server_ban_list_subscriptions_ban_list_id ON public.server_ban_list_subscriptions (ban_list_id);
CREATE INDEX IF NOT EXISTS idx_remote_ban_sources_sync_enabled ON public.remote_ban_sources (sync_enabled);
CREATE INDEX IF NOT EXISTS idx_ignored_steam_ids_steam_id ON public.ignored_steam_ids (steam_id);
INSERT INTO public.ban_lists (name, description)
VALUES (
        'Global',
        'Default global ban list for shared bans across servers'
    ) ON CONFLICT (name) DO NOTHING;