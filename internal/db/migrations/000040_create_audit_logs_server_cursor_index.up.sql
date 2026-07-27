CREATE INDEX CONCURRENTLY idx_audit_logs_server_cursor
    ON public.audit_logs (server_id, timestamp DESC, id DESC);
