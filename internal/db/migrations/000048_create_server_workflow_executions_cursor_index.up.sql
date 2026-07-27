CREATE INDEX CONCURRENTLY idx_server_workflow_executions_cursor
    ON public.server_workflow_executions (workflow_id, started_at DESC, id DESC);
