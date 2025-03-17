CREATE TABLE IF NOT EXISTS public.admin_chat_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id uuid,
    user_id uuid NOT NULL,
    message text NOT NULL,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_admin_chat_messages_server_id ON public.admin_chat_messages(server_id);
CREATE INDEX IF NOT EXISTS idx_admin_chat_messages_created_at ON public.admin_chat_messages(created_at);
CREATE INDEX IF NOT EXISTS idx_admin_chat_messages_server_created ON public.admin_chat_messages(server_id, created_at);