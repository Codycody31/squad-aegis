CREATE INDEX CONCURRENTLY idx_ban_evidence_ban_order
    ON public.ban_evidence (ban_id, COALESCE(event_time, created_at) DESC, id DESC);
