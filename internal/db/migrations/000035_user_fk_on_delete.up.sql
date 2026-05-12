ALTER TABLE public.audit_logs
    ADD COLUMN IF NOT EXISTS username_snapshot character varying(500);

UPDATE public.audit_logs al
SET username_snapshot = u.username
FROM public.users u
WHERE al.user_id = u.id
  AND al.username_snapshot IS NULL;

ALTER TABLE public.audit_logs
    DROP CONSTRAINT IF EXISTS fk_audit_logs_user_id_users_id;

ALTER TABLE public.audit_logs
    ADD CONSTRAINT fk_audit_logs_user_id_users_id
    FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE public.sessions
    DROP CONSTRAINT IF EXISTS fk_sessions_user_id_users_id;

ALTER TABLE public.sessions
    ADD CONSTRAINT fk_sessions_user_id_users_id
    FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE public.server_admins
    DROP CONSTRAINT IF EXISTS fk_server_admins_user_id_users_id;

ALTER TABLE public.server_admins
    ADD CONSTRAINT fk_server_admins_user_id_users_id
    FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;
