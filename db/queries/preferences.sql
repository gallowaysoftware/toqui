-- name: UpsertPreference :one
INSERT INTO user_preferences (user_id, key, value)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, key) DO UPDATE
    SET value = EXCLUDED.value,
        updated_at = NOW()
RETURNING *;

-- name: GetPreferences :many
SELECT * FROM user_preferences
WHERE user_id = $1
ORDER BY key;

-- name: DeletePreference :exec
DELETE FROM user_preferences
WHERE user_id = $1 AND key = $2;
