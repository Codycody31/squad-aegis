ALTER TABLE public.server_workflow_executions ADD COLUMN trigger_data JSONB;

CREATE INDEX idx_server_workflow_executions_trigger_data_gin ON public.server_workflow_executions USING GIN (trigger_data);

