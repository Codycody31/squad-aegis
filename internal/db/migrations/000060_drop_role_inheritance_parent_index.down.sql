CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_role_inheritance_parent
    ON public.role_inheritance (parent_role_id);
