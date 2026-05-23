-- name: RecordProInterest :one
INSERT INTO pro_interest (user_id, email)
VALUES (sqlc.arg(user_id), sqlc.arg(email))
ON CONFLICT (user_id) DO NOTHING
RETURNING *;

-- name: CountProInterest :one
SELECT COUNT(*) FROM pro_interest;

-- name: ListProInterest :many
SELECT * FROM pro_interest ORDER BY created_at DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);
