DROP INDEX IF EXISTS idx_player_rule_violations_player_eos_id;
DROP INDEX IF EXISTS idx_player_rule_violations_player_id;

-- Preserve EOS-only records by setting player_steam_id to 0 instead of deleting.
UPDATE public.player_rule_violations
SET player_steam_id = 0
WHERE player_steam_id IS NULL;

ALTER TABLE public.player_rule_violations
    ALTER COLUMN player_steam_id SET NOT NULL;

ALTER TABLE public.player_rule_violations
    DROP COLUMN IF EXISTS player_eos_id;

ALTER TABLE public.player_rule_violations
    DROP COLUMN IF EXISTS player_id;
