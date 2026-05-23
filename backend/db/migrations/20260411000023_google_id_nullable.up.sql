-- Allow users to sign up via Facebook (or other OAuth providers) without a Google ID.
ALTER TABLE users ALTER COLUMN google_id DROP NOT NULL;
