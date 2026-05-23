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

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: SearchUsers :many
SELECT * FROM users WHERE email ILIKE '%' || sqlc.arg(query)::text || '%' OR name ILIKE '%' || sqlc.arg(query)::text || '%'
ORDER BY created_at DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- Facebook + Apple OAuth queries were removed when the project transitioned
-- to self-hostable OSS (email+password default, Google OAuth optional).
-- The facebook_id column remains in the schema as a dead field; apple_sub +
-- subscription_tier + age_verified_at were dropped via the 20260524000001
-- cleanup migration.

-- name: IsUserAdmin :one
SELECT is_admin FROM users WHERE id = $1;

-- name: SetAdmin :exec
UPDATE users SET is_admin = sqlc.arg(is_admin), updated_at = NOW() WHERE id = sqlc.arg(user_id);

-- name: SeedAdminByEmail :exec
UPDATE users SET is_admin = true, updated_at = NOW() WHERE LOWER(email) = LOWER(sqlc.arg(email));

-- name: CreateUserWithPassword :one
INSERT INTO users (email, name, password_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserPasswordHash :one
SELECT id, password_hash FROM users WHERE email = $1;

-- name: UpdateUserPasswordHash :exec
UPDATE users SET password_hash = sqlc.arg(password_hash), updated_at = NOW()
WHERE id = sqlc.arg(user_id);
