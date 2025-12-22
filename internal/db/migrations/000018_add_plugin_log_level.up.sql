-- Add log_level column to plugin_instances table
-- Valid values: 'debug', 'info', 'warn', 'error'
-- Default is 'info' to store info, warn, and error logs
ALTER TABLE plugin_instances
ADD COLUMN log_level VARCHAR(10) NOT NULL DEFAULT 'info';

