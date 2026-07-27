CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ban_evidence_ban_id
    ON public.ban_evidence (ban_id);
