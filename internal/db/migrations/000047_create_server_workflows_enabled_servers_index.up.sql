CREATE INDEX CONCURRENTLY idx_server_workflows_enabled_servers
    ON public.server_workflows (server_id)
    WHERE enabled;
