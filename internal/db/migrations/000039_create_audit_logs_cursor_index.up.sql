CREATE INDEX CONCURRENTLY idx_audit_logs_cursor
    ON public.audit_logs (timestamp DESC, id DESC);
