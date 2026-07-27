CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_workflow_executions_status
    ON public.server_workflow_executions (status);
