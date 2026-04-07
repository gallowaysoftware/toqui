-- name: CreateTrip :one
INSERT INTO trips (user_id, title, description, start_date, end_date)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetTripByID :one
SELECT * FROM trips WHERE id = $1 AND user_id = $2;

-- name: GetTripByIDOrCollaborator :one
SELECT t.* FROM trips t
WHERE t.id = $1
  AND (t.user_id = $2 OR EXISTS (
    SELECT 1 FROM trip_collaborators tc
    WHERE tc.trip_id = t.id AND tc.user_id = $2 AND tc.accepted_at IS NOT NULL
  ));

-- name: ListTripsByUser :many
SELECT * FROM trips
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListTripsByUserAndStatus :many
SELECT * FROM trips
WHERE user_id = $1 AND status = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountTripsByUser :one
SELECT COUNT(*) FROM trips WHERE user_id = $1;

-- name: CountTripsByUserAndStatus :one
SELECT COUNT(*) FROM trips WHERE user_id = $1 AND status = $2;

-- name: UpdateTrip :one
UPDATE trips
SET title = COALESCE(NULLIF(sqlc.arg(title)::text, ''), title),
    description = COALESCE(sqlc.arg(description), description),
    status = COALESCE(NULLIF(sqlc.arg(status)::text, ''), status),
    start_date = COALESCE(sqlc.arg(start_date), start_date),
    end_date = COALESCE(sqlc.arg(end_date), end_date),
    updated_at = NOW()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id)
RETURNING *;

-- name: UpdateTripDestination :execresult
UPDATE trips
SET destination_country = $2, updated_at = NOW()
WHERE id = $1 AND user_id = $3;

-- name: UpdateTripDestinations :execresult
UPDATE trips
SET destination_countries = sqlc.arg(destination_countries)::text[],
    destination_country = COALESCE(NULLIF(sqlc.arg(primary_country)::text, ''), destination_country),
    updated_at = NOW()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: DeleteTrip :exec
DELETE FROM trips WHERE id = $1 AND user_id = $2;

-- name: EnableTripSharing :one
UPDATE trips SET share_token = sqlc.arg(share_token), updated_at = NOW()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id)
RETURNING *;

-- name: DisableTripSharing :one
UPDATE trips SET share_token = NULL, updated_at = NOW()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id)
RETURNING *;

-- name: GetTripByShareToken :one
SELECT * FROM trips WHERE share_token = sqlc.arg(share_token);

-- name: CountActiveTrips :one
SELECT COUNT(*) FROM trips WHERE status = 'active';

-- name: StartTripTrial :exec
UPDATE trips SET trial_started_at = NOW(), trial_ends_at = NOW() + INTERVAL '3 days', updated_at = NOW()
WHERE id = $1 AND trial_started_at IS NULL;

-- name: IsTripTrialActive :one
SELECT COALESCE(trial_ends_at > NOW(), false)::boolean AS active FROM trips WHERE id = $1;
