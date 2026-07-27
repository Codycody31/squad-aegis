DO $$
DECLARE
    duplicate_execution_id uuid;
    duplicate_count bigint;
BEGIN
    SELECT execution_id, COUNT(*)
    INTO duplicate_execution_id, duplicate_count
    FROM public.server_workflow_executions
    GROUP BY execution_id
    HAVING COUNT(*) > 1
    ORDER BY COUNT(*) DESC, execution_id
    LIMIT 1;

    IF duplicate_execution_id IS NOT NULL THEN
        RAISE EXCEPTION
            'cannot create unique workflow execution index: execution_id % appears % times; resolve duplicate server_workflow_executions rows before retrying',
            duplicate_execution_id,
            duplicate_count
            USING ERRCODE = '23505';
    END IF;
END
$$;
