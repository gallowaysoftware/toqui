-- Apple Sign-In: link users to Apple's stable user identifier (`sub` claim).
-- Apple's `sub` is unique per (team, user); we store it as TEXT and enforce
-- uniqueness so re-logins via Apple resolve to the same Toqui user.
ALTER TABLE users ADD COLUMN apple_sub TEXT UNIQUE;

-- Partial index keeps the index small (only users who have linked Apple).
CREATE INDEX idx_users_apple_sub ON users (apple_sub) WHERE apple_sub IS NOT NULL;
