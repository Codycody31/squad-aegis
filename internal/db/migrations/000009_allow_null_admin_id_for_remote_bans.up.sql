-- Allow admin_id and server_id to be NULL for remote/global bans
ALTER TABLE public.server_bans 
    ALTER COLUMN admin_id DROP NOT NULL;

ALTER TABLE public.server_bans 
    ALTER COLUMN server_id DROP NOT NULL;

-- Drop the existing foreign key constraints
ALTER TABLE public.server_bans
    DROP CONSTRAINT IF EXISTS fk_server_bans_admin_id_users_id;

ALTER TABLE public.server_bans
    DROP CONSTRAINT IF EXISTS fk_server_bans_server_id_servers_id;

-- Add the foreign key constraints back but allow NULL values
ALTER TABLE public.server_bans
    ADD CONSTRAINT fk_server_bans_admin_id_users_id 
    FOREIGN KEY (admin_id) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE public.server_bans
    ADD CONSTRAINT fk_server_bans_server_id_servers_id 
    FOREIGN KEY (server_id) REFERENCES public.servers(id) ON DELETE SET NULL;
