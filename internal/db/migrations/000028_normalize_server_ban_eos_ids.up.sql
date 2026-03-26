UPDATE server_bans
SET eos_id = lower(trim(eos_id))
WHERE eos_id IS NOT NULL;
