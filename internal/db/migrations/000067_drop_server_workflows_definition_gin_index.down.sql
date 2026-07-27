CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_workflows_definition_gin
    ON public.server_workflows USING gin (definition);
