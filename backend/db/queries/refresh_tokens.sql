-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, jti, family, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetRefreshTokenByJTI :one
SELECT * FROM refresh_tokens
WHERE jti = $1;

-- name: GetRefreshTokenByJTIForUpdate :one
-- Row-locks the refresh_tokens row for the duration of the enclosing
-- transaction so concurrent RefreshToken RPCs serialize on rotation.
-- Must be used inside a transaction; closes the TOCTOU window where two
-- parallel refreshes with the same JTI could both observe revoked=false.
SELECT * FROM refresh_tokens
WHERE jti = $1
FOR UPDATE;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked = true
WHERE jti = $1;

-- name: RevokeRefreshTokenFamily :exec
UPDATE refresh_tokens
SET revoked = true
WHERE family = $1;

-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens
WHERE expires_at < now();
