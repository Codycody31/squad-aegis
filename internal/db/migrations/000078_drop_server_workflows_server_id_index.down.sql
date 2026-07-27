CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_workflows_server_id
    ON public.server_workflows (server_id);
