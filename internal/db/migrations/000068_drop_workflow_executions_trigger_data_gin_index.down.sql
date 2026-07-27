CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_workflow_executions_trigger_data_gin
    ON public.server_workflow_executions USING gin (trigger_data);
