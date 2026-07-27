CREATE INDEX CONCURRENTLY idx_server_workflows_cursor
    ON public.server_workflows (server_id, created_at DESC, id DESC);
