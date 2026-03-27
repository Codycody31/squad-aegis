ALTER TABLE public.player_rule_violations
    ADD COLUMN IF NOT EXISTS player_id TEXT;

ALTER TABLE public.player_rule_violations
    ADD COLUMN IF NOT EXISTS player_eos_id TEXT NOT NULL DEFAULT '';

UPDATE public.player_rule_violations
SET
    player_id = COALESCE(NULLIF(player_id, ''), player_steam_id::text),
    player_eos_id = COALESCE(player_eos_id, '')
WHERE player_id IS NULL OR player_id = '' OR player_eos_id IS NULL;

ALTER TABLE public.player_rule_violations
    ALTER COLUMN player_id SET NOT NULL;

ALTER TABLE public.player_rule_violations
    ALTER COLUMN player_steam_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_player_rule_violations_player_id
    ON public.player_rule_violations(player_id);

CREATE INDEX IF NOT EXISTS idx_player_rule_violations_player_eos_id
    ON public.player_rule_violations(player_eos_id)
    WHERE player_eos_id != '';
