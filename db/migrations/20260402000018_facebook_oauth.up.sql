ALTER TABLE users ADD COLUMN facebook_id TEXT UNIQUE;
CREATE INDEX idx_users_facebook_id ON users(facebook_id);
