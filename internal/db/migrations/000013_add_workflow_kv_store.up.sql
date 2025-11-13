-- Create workflow KV store table for persistent key-value storage per workflow
CREATE TABLE public.server_workflow_kv_store (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id uuid NOT NULL,
    key VARCHAR(255) NOT NULL,
    value JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_server_workflow_kv_store_workflow_id FOREIGN KEY (workflow_id) REFERENCES public.server_workflows(id) ON DELETE CASCADE,
    UNIQUE(workflow_id, key)
);

-- Create indexes for efficient queries
CREATE INDEX idx_server_workflow_kv_store_workflow_id ON public.server_workflow_kv_store(workflow_id);
CREATE INDEX idx_server_workflow_kv_store_key ON public.server_workflow_kv_store(workflow_id, key);
CREATE INDEX idx_server_workflow_kv_store_value_gin ON public.server_workflow_kv_store USING GIN (value);
