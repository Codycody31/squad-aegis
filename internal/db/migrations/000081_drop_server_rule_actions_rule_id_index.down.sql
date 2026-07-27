CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_server_rule_actions_rule_id
    ON public.server_rule_actions (rule_id);
