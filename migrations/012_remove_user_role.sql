-- Remove the unused role column from users table.
-- The role was always hardcoded to 'member' and never used for authorization.
ALTER TABLE users DROP COLUMN IF EXISTS role;
