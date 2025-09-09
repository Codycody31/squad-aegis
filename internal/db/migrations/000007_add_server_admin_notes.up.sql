-- Add notes column to server_admins table for admin role assignment notes
ALTER TABLE "public"."server_admins" 
ADD COLUMN "notes" text;
