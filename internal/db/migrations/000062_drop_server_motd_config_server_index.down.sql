CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_motd_config_server_id
    ON public.server_motd_config (server_id);
