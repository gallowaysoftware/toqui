ALTER TABLE trips ADD COLUMN share_token VARCHAR(32) UNIQUE;
CREATE INDEX idx_trips_share_token ON trips(share_token) WHERE share_token IS NOT NULL;
