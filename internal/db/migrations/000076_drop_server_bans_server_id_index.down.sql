CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_bans_server_id
    ON public.server_bans (server_id);
