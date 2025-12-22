DROP INDEX IF EXISTS idx_server_workflow_executions_trigger_data_gin;

ALTER TABLE public.server_workflow_executions DROP COLUMN IF EXISTS trigger_data;

