CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_workflow_variables_workflow_id
    ON public.server_workflow_variables (workflow_id);
