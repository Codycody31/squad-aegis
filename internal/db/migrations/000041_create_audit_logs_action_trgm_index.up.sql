CREATE INDEX CONCURRENTLY idx_audit_logs_action_trgm
    ON public.audit_logs USING gin (lower(action) gin_trgm_ops);
