CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_workflow_kv_store_value_gin
    ON public.server_workflow_kv_store USING gin (value);
