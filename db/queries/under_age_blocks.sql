-- name: RecordUnderAgeBlock :one
-- Record that an OAuth identity was refused due to the under-18 age gate.
--
-- The `RETURNING id` is what makes this query the serialization point for
-- concurrent verify-age requests from the same user (W1 race window
-- closed). When two requests race:
--
--   - The first INSERT acquires the row, RETURNING returns its id, and
--     the handler proceeds with lifecycle.DeleteUser + audit.
--   - The second INSERT hits ON CONFLICT, RETURNING returns NO rows,
--     and sqlc surfaces this as `pgx.ErrNoRows`. The handler treats
--     that as "another concurrent request already handled this user"
--     and short-circuits without firing a duplicate audit event or a
--     redundant DeleteUser call.
--
-- This relies on the `email_sha256 UNIQUE` constraint, so the
-- serialization is per-email — the right granularity, since two
-- distinct under-18 users with the same email is impossible (the
-- users.email column is itself UNIQUE at insert time and the email
-- never changes after creation).
INSERT INTO under_age_blocks (email_sha256, oauth_provider)
VALUES (sqlc.arg(email_sha256), sqlc.arg(oauth_provider))
ON CONFLICT (email_sha256) DO NOTHING
RETURNING id;

-- name: IsEmailUnderAgeBlocked :one
-- Returns true iff the given email hash was previously refused.
-- Called from the OAuth login handlers BEFORE the user upsert, so a
-- refused person can't simply sign in again with the same
-- Google/Facebook/Apple identity.
SELECT EXISTS (
    SELECT 1 FROM under_age_blocks WHERE email_sha256 = sqlc.arg(email_sha256)
);
