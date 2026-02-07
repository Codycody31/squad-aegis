-- Migration 000023: Add DEFAULT gen_random_uuid() to server_roles.id
-- The server_roles table was created in 000001 without a DEFAULT on the id column.
-- Newer code paths (e.g., creating roles from templates) omit the id in INSERT
-- statements, expecting the database to generate it.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

ALTER TABLE server_roles
    ALTER COLUMN id SET DEFAULT gen_random_uuid();
