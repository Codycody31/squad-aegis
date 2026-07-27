CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_plugin_data_plugin_instance_id
    ON public.plugin_data (plugin_instance_id);
