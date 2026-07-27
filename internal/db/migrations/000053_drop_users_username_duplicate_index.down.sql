CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS users_users_username_key
    ON public.users (username);
