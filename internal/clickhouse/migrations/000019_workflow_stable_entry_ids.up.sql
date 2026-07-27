ALTER TABLE squad_aegis.workflow_execution_logs
ADD COLUMN IF NOT EXISTS entry_id UUID DEFAULT reinterpretAsUUID(MD5(concat(
    toString(server_id), '|',
    toString(workflow_id), '|',
    toString(execution_id), '|',
    toString(step_order), '|',
    toString(event_time), '|',
    step_name, '|',
    step_type, '|',
    step_status
)));

--migration:split

ALTER TABLE squad_aegis.workflow_log_messages
ADD COLUMN IF NOT EXISTS entry_id UUID DEFAULT reinterpretAsUUID(MD5(concat(
    toString(server_id), '|',
    toString(workflow_id), '|',
    toString(execution_id), '|',
    toString(log_time), '|',
    step_id, '|',
    step_name, '|',
    log_level, '|',
    message
)));
