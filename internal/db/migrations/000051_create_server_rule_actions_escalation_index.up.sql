CREATE INDEX CONCURRENTLY idx_server_rule_actions_escalation
    ON public.server_rule_actions (rule_id, violation_count DESC, id);
