CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_workflow_kv_store_key
    ON public.server_workflow_kv_store (workflow_id, key);
