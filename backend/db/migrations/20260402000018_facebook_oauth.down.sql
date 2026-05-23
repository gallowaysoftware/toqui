DROP INDEX IF EXISTS idx_users_facebook_id;
ALTER TABLE users DROP COLUMN IF EXISTS facebook_id;
