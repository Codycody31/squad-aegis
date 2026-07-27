CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_workflow_executions_workflow_id
    ON public.server_workflow_executions (workflow_id);
