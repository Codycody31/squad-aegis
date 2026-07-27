CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_permissions_code
    ON public.permissions (code);
