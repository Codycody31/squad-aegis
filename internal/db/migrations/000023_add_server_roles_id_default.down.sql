-- Revert migration 000023: Remove DEFAULT from server_roles.id

ALTER TABLE server_roles
    ALTER COLUMN id DROP DEFAULT;
