CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_role_template_permissions_template
    ON public.role_template_permissions (role_template_id);
