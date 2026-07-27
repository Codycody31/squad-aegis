CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_admins_user_id
    ON public.server_admins (user_id);
