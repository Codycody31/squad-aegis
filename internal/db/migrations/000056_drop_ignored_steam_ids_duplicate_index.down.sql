CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ignored_steam_ids_steam_id
    ON public.ignored_steam_ids (steam_id);
