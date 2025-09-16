CREATE TABLE public.server_workflows (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id uuid NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,
    definition JSONB NOT NULL,
    created_by uuid NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE public.server_workflow_executions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    status VARCHAR(50) NOT NULL CHECK (status IN ('RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    error_message TEXT,
    CONSTRAINT fk_server_workflow_executions_workflow_id FOREIGN KEY (workflow_id) REFERENCES public.server_workflows(id) ON DELETE CASCADE
);

CREATE TABLE public.server_workflow_variables (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id uuid NOT NULL,
    name VARCHAR(255) NOT NULL,
    value JSONB NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_server_workflow_variables_workflow_id FOREIGN KEY (workflow_id) REFERENCES public.server_workflows(id) ON DELETE CASCADE,
    UNIQUE(workflow_id, name)
);

CREATE INDEX idx_server_workflows_server_id ON public.server_workflows(server_id);
CREATE INDEX idx_server_workflows_enabled ON public.server_workflows(enabled);
CREATE INDEX idx_server_workflow_executions_workflow_id ON public.server_workflow_executions(workflow_id);
CREATE INDEX idx_server_workflow_executions_status ON public.server_workflow_executions(status);
CREATE INDEX idx_server_workflow_executions_started_at ON public.server_workflow_executions(started_at);
CREATE INDEX idx_server_workflow_variables_workflow_id ON public.server_workflow_variables(workflow_id);

CREATE INDEX idx_server_workflows_definition_gin ON public.server_workflows USING GIN (definition);
