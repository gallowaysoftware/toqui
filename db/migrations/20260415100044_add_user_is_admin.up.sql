-- Add is_admin column to users table for database-driven admin authorization.
-- Replaces the ADMIN_EMAILS environment variable string-matching approach
-- which was a privilege escalation risk (env var leaks grant admin access).
ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT false;
