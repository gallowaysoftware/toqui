-- Add email verification to waitlist
ALTER TABLE waitlist ADD COLUMN verify_token TEXT UNIQUE;
ALTER TABLE waitlist ADD COLUMN verified_at TIMESTAMPTZ;

CREATE INDEX idx_waitlist_verify_token ON waitlist(verify_token) WHERE verify_token IS NOT NULL;

-- Backfill: treat existing entries as already verified
UPDATE waitlist SET verified_at = signed_up_at WHERE verified_at IS NULL;
