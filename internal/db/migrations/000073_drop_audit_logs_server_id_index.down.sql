CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_audit_logs_server_id
    ON public.audit_logs (server_id);
