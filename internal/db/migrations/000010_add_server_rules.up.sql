ALTER TABLE public.server_bans DROP CONSTRAINT IF EXISTS fk_server_bans_rule_id_server_rules_id;
DROP TABLE IF EXISTS public.server_rules;
CREATE TABLE public.server_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id uuid NOT NULL,
    parent_id uuid,
    display_order INT NOT NULL DEFAULT 0,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(server_id, created_at)
);
CREATE TABLE public.server_rule_actions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id uuid NOT NULL,
    violation_count INT NOT NULL,
    action_type VARCHAR(255) NOT NULL CHECK (action_type IN ('WARN', 'KICK', 'BAN')),
    duration_minutes INT,
    message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(rule_id, violation_count)
);
CREATE TABLE public.player_rule_violations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id uuid NOT NULL,
    player_steam_id bigint NOT NULL,
    rule_id uuid NOT NULL,
    admin_user_id uuid,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_server_rules_parent_id ON public.server_rules(parent_id);
CREATE INDEX idx_server_rules_server_id ON public.server_rules(server_id);
CREATE INDEX idx_server_rule_actions_rule_id ON public.server_rule_actions(rule_id);
CREATE INDEX idx_player_rule_violations_player_steam_id ON public.player_rule_violations(player_steam_id);
CREATE INDEX idx_player_rule_violations_rule_id ON public.player_rule_violations(rule_id);
CREATE INDEX idx_player_rule_violations_server_id ON public.player_rule_violations(server_id);