CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ban_lists_name
    ON public.ban_lists (name);
