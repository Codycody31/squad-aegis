CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_audit_logs_timestamp
    ON public.audit_logs (timestamp);
