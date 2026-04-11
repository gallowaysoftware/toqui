-- name: CompleteTrip :exec
UPDATE trips
SET status = 'completed',
    completed_at = NOW(),
    archive_after = NOW() + INTERVAL '90 days',
    updated_at = NOW()
WHERE id = $1 AND user_id = $2;

-- name: ArchiveTrip :exec
UPDATE trips
SET status = 'archived', archived_at = NOW(), updated_at = NOW()
WHERE id = $1 AND user_id = $2;

-- name: GetTripsToArchive :many
SELECT id, user_id
FROM trips
WHERE status = 'completed'
  AND archive_after IS NOT NULL
  AND archive_after < NOW()
  AND archived_at IS NULL;

-- name: DeleteTripByUser :exec
DELETE FROM trips WHERE id = $1 AND user_id = $2;

-- name: DeleteUserByID :exec
DELETE FROM users WHERE id = $1;

-- name: GetAllTripIDsForUser :many
SELECT id FROM trips WHERE user_id = $1;

-- name: CreateDeletionRequest :one
INSERT INTO deletion_requests (user_id)
VALUES ($1)
RETURNING *;

-- name: CompleteDeletionRequest :exec
UPDATE deletion_requests
SET status = 'completed', completed_at = NOW()
WHERE id = $1;

-- name: GetPendingDeletionRequests :many
SELECT id, user_id, requested_at
FROM deletion_requests
WHERE status = 'pending'
ORDER BY requested_at ASC;

-- name: CreateExportRequest :one
INSERT INTO export_requests (user_id)
VALUES ($1)
RETURNING *;

-- name: CompleteExportRequest :exec
UPDATE export_requests
SET status = 'completed', completed_at = NOW(), download_url = $2, expires_at = $3
WHERE id = $1;

-- name: GetUserExportRequests :many
SELECT id, requested_at, completed_at, download_url, expires_at, status
FROM export_requests
WHERE user_id = $1
ORDER BY requested_at DESC
LIMIT 10;

-- name: SetDeletionRequestProcessing :exec
UPDATE deletion_requests
SET status = 'processing'
WHERE id = $1;

-- name: GetStaleDeletionRequests :many
SELECT id, user_id, requested_at, retry_count
FROM deletion_requests
WHERE status = 'processing'
  AND requested_at < NOW() - INTERVAL '1 hour'
ORDER BY requested_at ASC;

-- name: IncrementDeletionRetryCount :exec
UPDATE deletion_requests
SET retry_count = retry_count + 1, status = 'processing'
WHERE id = $1;

-- name: FailDeletionRequest :exec
UPDATE deletion_requests
SET status = 'failed'
WHERE id = $1;
