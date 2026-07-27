CREATE UNIQUE INDEX CONCURRENTLY uq_server_workflow_executions_execution_id
    ON public.server_workflow_executions (execution_id);
