-- name: UpsertUserByGoogleID :one
INSERT INTO users (google_id, email, name, avatar_url)
VALUES ($1, $2, $3, $4)
ON CONFLICT (google_id)
DO UPDATE SET email = EXCLUDED.email, name = EXCLUDED.name, avatar_url = EXCLUDED.avatar_url, updated_at = NOW()
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByGoogleID :one
SELECT * FROM users WHERE google_id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: SetUserDefaultPersona :one
UPDATE users SET default_persona_id = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: GetUserDefaultPersona :one
SELECT default_persona_id FROM users WHERE id = $1;

-- name: GetUserSubscriptionTier :one
SELECT COALESCE(subscription_tier, 'free') FROM users WHERE id = $1;

-- name: SetAgeVerified :exec
UPDATE users SET age_verified_at = NOW(), updated_at = NOW() WHERE id = $1;

-- name: IsAgeVerified :one
SELECT COALESCE(age_verified_at IS NOT NULL, false)::boolean AS verified FROM users WHERE id = $1;
