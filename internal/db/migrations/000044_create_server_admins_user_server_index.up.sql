CREATE INDEX CONCURRENTLY idx_server_admins_user_server
    ON public.server_admins (user_id, server_id);
