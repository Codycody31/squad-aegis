ALTER TABLE squad_aegis.workflow_execution_logs
DROP COLUMN IF EXISTS entry_id;

--migration:split

ALTER TABLE squad_aegis.workflow_log_messages
DROP COLUMN IF EXISTS entry_id;
