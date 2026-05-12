ALTER TABLE public.audit_logs
    DROP CONSTRAINT IF EXISTS fk_audit_logs_user_id_users_id;

ALTER TABLE public.audit_logs
    ADD CONSTRAINT fk_audit_logs_user_id_users_id
    FOREIGN KEY (user_id) REFERENCES public.users(id);

ALTER TABLE public.audit_logs
    DROP COLUMN IF EXISTS username_snapshot;

ALTER TABLE public.sessions
    DROP CONSTRAINT IF EXISTS fk_sessions_user_id_users_id;

ALTER TABLE public.sessions
    ADD CONSTRAINT fk_sessions_user_id_users_id
    FOREIGN KEY (user_id) REFERENCES public.users(id);

ALTER TABLE public.server_admins
    DROP CONSTRAINT IF EXISTS fk_server_admins_user_id_users_id;

ALTER TABLE public.server_admins
    ADD CONSTRAINT fk_server_admins_user_id_users_id
    FOREIGN KEY (user_id) REFERENCES public.users(id);
