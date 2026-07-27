CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_role_permissions_role
    ON public.server_role_permissions (server_role_id);
