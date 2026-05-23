-- name: CreateFeedback :one
INSERT INTO feedback (user_id, type, message, context, page, trip_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListFeedback :many
SELECT f.*, u.email, u.name
FROM feedback f
JOIN users u ON f.user_id = u.id
ORDER BY f.created_at DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: CountFeedback :one
SELECT COUNT(*) FROM feedback;

-- name: ListFeedbackByType :many
SELECT f.*, u.email, u.name
FROM feedback f
JOIN users u ON f.user_id = u.id
WHERE f.type = $1
ORDER BY f.created_at DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: ListFeedbackByUser :many
-- Returns the feedback rows belonging to a single user, used by the GDPR
-- Article 20 export (lifecycle.Service.ExportUserData). Joining users
-- isn't needed here — the export already includes the user profile, and
-- the requester knows their own email/name.
SELECT *
FROM feedback
WHERE user_id = $1
ORDER BY created_at DESC;
