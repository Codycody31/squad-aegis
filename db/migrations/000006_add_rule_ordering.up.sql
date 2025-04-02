ALTER TABLE server_rules ADD COLUMN order_key VARCHAR(50);
UPDATE server_rules SET order_key = id::text WHERE order_key IS NULL;