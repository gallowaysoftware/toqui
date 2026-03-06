-- name: AddToWaitlist :one
INSERT INTO waitlist (email)
VALUES (sqlc.arg(email))
ON CONFLICT (email) DO NOTHING
RETURNING *;

-- name: GetWaitlistByEmail :one
SELECT * FROM waitlist WHERE email = sqlc.arg(email);

-- name: GetWaitlistByInviteCode :one
SELECT * FROM waitlist WHERE invite_code = sqlc.arg(invite_code);

-- name: CountWaitlistAhead :one
SELECT COUNT(*) FROM waitlist
WHERE signed_up_at < sqlc.arg(signed_up_at)
AND accepted_at IS NULL;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: MarkWaitlistAccepted :exec
UPDATE waitlist SET accepted_at = NOW()
WHERE email = sqlc.arg(email);
