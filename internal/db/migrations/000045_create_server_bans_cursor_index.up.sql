CREATE INDEX CONCURRENTLY idx_server_bans_cursor
    ON public.server_bans (server_id, created_at DESC, id DESC);
