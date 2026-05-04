-- name: RecordUnderAgeBlock :exec
-- Record that an OAuth identity was refused due to the under-18 age gate.
-- ON CONFLICT DO NOTHING so re-attempts are idempotent — the table tracks
-- "this email was refused", not "how many times it tried to retry".
INSERT INTO under_age_blocks (email_sha256, oauth_provider)
VALUES (sqlc.arg(email_sha256), sqlc.arg(oauth_provider))
ON CONFLICT (email_sha256) DO NOTHING;

-- name: IsEmailUnderAgeBlocked :one
-- Returns true iff the given email hash was previously refused.
-- Called from the OAuth login handlers BEFORE the user upsert, so a
-- refused person can't simply retry by signing in again with the same
-- Google/Facebook/Apple identity.
SELECT EXISTS (
    SELECT 1 FROM under_age_blocks WHERE email_sha256 = sqlc.arg(email_sha256)
);
