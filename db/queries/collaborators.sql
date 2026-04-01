-- name: ListCollaborators :many
SELECT * FROM trip_collaborators
WHERE trip_id = $1
ORDER BY invited_at;

-- name: AddCollaborator :one
INSERT INTO trip_collaborators (trip_id, email, role, invite_token, invited_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: AcceptInvite :one
UPDATE trip_collaborators
SET user_id = $2, accepted_at = NOW()
WHERE invite_token = $1 AND accepted_at IS NULL
RETURNING *;

-- name: RemoveCollaborator :exec
DELETE FROM trip_collaborators
WHERE trip_id = $1 AND email = $2;

-- name: GetCollaboratorByToken :one
SELECT * FROM trip_collaborators
WHERE invite_token = $1;

-- name: GetCollaboratorAccess :one
SELECT * FROM trip_collaborators
WHERE trip_id = $1 AND user_id = $2;

-- name: ListSharedTrips :many
SELECT t.* FROM trips t
INNER JOIN trip_collaborators tc ON tc.trip_id = t.id
WHERE tc.user_id = $1 AND tc.accepted_at IS NOT NULL
ORDER BY t.created_at DESC;

-- name: CountCollaboratorsByTrip :one
SELECT COUNT(*) FROM trip_collaborators WHERE trip_id = $1;

-- name: GetTripOwner :one
SELECT user_id FROM trips WHERE id = $1;
