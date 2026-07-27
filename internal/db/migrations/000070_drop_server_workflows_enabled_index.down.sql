CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_workflows_enabled
    ON public.server_workflows (enabled);
