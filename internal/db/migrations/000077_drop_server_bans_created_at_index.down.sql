CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_bans_created_at
    ON public.server_bans (created_at);
