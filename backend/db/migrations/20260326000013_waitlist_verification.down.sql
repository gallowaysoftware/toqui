DROP INDEX IF EXISTS idx_waitlist_verify_token;
ALTER TABLE waitlist DROP COLUMN IF EXISTS verify_token;
ALTER TABLE waitlist DROP COLUMN IF EXISTS verified_at;
