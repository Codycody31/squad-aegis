CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_workflow_executions_started_at
    ON public.server_workflow_executions (started_at);
