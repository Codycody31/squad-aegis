CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_admins_server_id
    ON public.server_admins (server_id);
