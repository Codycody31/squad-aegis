-- Add expires_at column to server_admins table to support temporary admin roles
ALTER TABLE "public"."server_admins" 
ADD COLUMN "expires_at" timestamp without time zone;

-- Add index for expires_at to optimize queries for expired admins
CREATE INDEX "idx_server_admins_expires_at" ON "public"."server_admins" ("expires_at");
