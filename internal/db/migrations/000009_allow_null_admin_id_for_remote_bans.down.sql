-- Revert admin_id and server_id to NOT NULL and recreate original constraints
ALTER TABLE public.server_bans
    DROP CONSTRAINT IF EXISTS fk_server_bans_admin_id_users_id;

ALTER TABLE public.server_bans
    DROP CONSTRAINT IF EXISTS fk_server_bans_server_id_servers_id;

ALTER TABLE public.server_bans 
    ALTER COLUMN admin_id SET NOT NULL;

ALTER TABLE public.server_bans 
    ALTER COLUMN server_id SET NOT NULL;

ALTER TABLE public.server_bans
    ADD CONSTRAINT fk_server_bans_admin_id_users_id 
    FOREIGN KEY (admin_id) REFERENCES public.users(id);

ALTER TABLE public.server_bans
    ADD CONSTRAINT fk_server_bans_server_id_servers_id 
    FOREIGN KEY (server_id) REFERENCES public.servers(id);
