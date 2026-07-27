CREATE INDEX CONCURRENTLY idx_audit_logs_username_snapshot_trgm
    ON public.audit_logs USING gin (lower(COALESCE(username_snapshot, '')) gin_trgm_ops);
