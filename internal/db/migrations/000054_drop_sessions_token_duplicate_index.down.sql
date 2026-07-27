CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS sessions_sessions_token_key
    ON public.sessions (token);
