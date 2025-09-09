-- Remove notes column from server_admins table
ALTER TABLE "public"."server_admins" 
DROP COLUMN IF EXISTS "notes";
