DROP INDEX IF EXISTS idx_trips_share_token;
ALTER TABLE trips DROP COLUMN IF EXISTS share_token;
