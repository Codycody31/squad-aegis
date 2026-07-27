CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_workflow_kv_store_workflow_id
    ON public.server_workflow_kv_store (workflow_id);
