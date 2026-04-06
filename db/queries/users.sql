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

-- name: SetUserSubscriptionTier :exec
UPDATE users SET subscription_tier = $1, updated_at = NOW() WHERE email = $2;

-- name: SetUserSubscriptionTierByID :exec
UPDATE users SET subscription_tier = sqlc.arg(tier), updated_at = NOW() WHERE id = sqlc.arg(user_id);

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: SearchUsers :many
SELECT * FROM users WHERE email ILIKE '%' || sqlc.arg(query)::text || '%' OR name ILIKE '%' || sqlc.arg(query)::text || '%'
ORDER BY created_at DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: GetUserByFacebookID :one
SELECT * FROM users WHERE facebook_id = $1;

-- name: UpdateUserFacebookID :exec
UPDATE users SET facebook_id = $2, updated_at = NOW() WHERE id = $1;

-- name: CreateUserWithFacebook :one
INSERT INTO users (email, name, facebook_id, avatar_url)
VALUES ($1, $2, $3, $4)
RETURNING *;
