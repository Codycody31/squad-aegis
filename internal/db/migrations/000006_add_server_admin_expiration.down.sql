-- Remove expires_at column from server_admins table
DROP INDEX IF EXISTS "idx_server_admins_expires_at";
ALTER TABLE "public"."server_admins" 
DROP COLUMN IF EXISTS "expires_at";
