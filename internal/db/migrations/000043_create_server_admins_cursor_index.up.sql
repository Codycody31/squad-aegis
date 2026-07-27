CREATE INDEX CONCURRENTLY idx_server_admins_cursor
    ON public.server_admins (server_id, created_at DESC, id DESC);
