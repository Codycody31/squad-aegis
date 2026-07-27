CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_ban_list_subscriptions_server_id
    ON public.server_ban_list_subscriptions (server_id);
